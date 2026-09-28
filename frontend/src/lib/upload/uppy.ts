import type Uppy from "@uppy/core";
import { toast } from "svelte-sonner";
import { uploadState } from "$lib/stores/upload.svelte.js";
import { getCsrfToken } from "$lib/api";

let instance: Uppy | null = null;
let emaFilterFn: ((newValue: number, oldValue: number, halfLife: number, dt: number) => number) | null = null;

// Raw progress buffer: not reactive. Uppy events write here freely.
// The flush timer reads from here and batch-updates reactive uploadState.
const rawProgress = new Map<string, number>();
let progressDirty = false;

// Speed tracking (non-reactive)
let flushInterval: ReturnType<typeof setInterval> | null = null;
let prevTotalUploaded = 0;
let prevFlushTime = 0;
let smoothedSpeed = 0;
let flushSamples = 0;
const SPEED_HALF_LIFE = 5000;
const FLUSH_INTERVAL_MS = 500;
// Withhold the ETA until the speed EMA has ramped past its cold seed (~5s of
// progress samples). Without this, big tiny-file folders flash a wildly
// pessimistic byte-rate ETA at startup before the connection pool warms up.
const ETA_WARMUP_SAMPLES = 10;

function startFlushTimer() {
	if (flushInterval) return;
	prevFlushTime = Date.now();
	prevTotalUploaded = uploadState.totalBytesUploaded;
	smoothedSpeed = 0;
	flushSamples = 0;

	flushInterval = setInterval(() => {
		if (!progressDirty) return;
		progressDirty = false;

		const now = Date.now();
		const dt = now - prevFlushTime;
		prevFlushTime = now;

		// Flush per-file progress to reactive state
		for (const [id, bytesUploaded] of rawProgress) {
			uploadState.updateProgress(id, bytesUploaded);
		}

		// Compute speed with EMA smoothing
		const totalUploaded = uploadState.totalBytesUploaded;
		const bytesDelta = totalUploaded - prevTotalUploaded;
		prevTotalUploaded = totalUploaded;

		if (dt > 0) {
			const instantSpeed = (bytesDelta / dt) * 1000;
			smoothedSpeed = smoothedSpeed === 0
				? instantSpeed
				: emaFilterFn!(instantSpeed, smoothedSpeed, SPEED_HALF_LIFE, dt);
		}

		// Compute ETA: withheld during warmup so the cold-seeded byte rate doesn't
		// flash a wildly pessimistic estimate; the panel shows "estimating…" until then.
		flushSamples++;
		const remaining = uploadState.totalBytes - totalUploaded;
		const warm = flushSamples >= ETA_WARMUP_SAMPLES;
		const eta = warm && smoothedSpeed > 0 ? remaining / smoothedSpeed : null;

		uploadState.updateSpeedAndEta(smoothedSpeed, eta);
	}, FLUSH_INTERVAL_MS);
}

function stopFlushTimer() {
	if (flushInterval) {
		clearInterval(flushInterval);
		flushInterval = null;
	}
	rawProgress.clear();
	progressDirty = false;
	smoothedSpeed = 0;
	flushSamples = 0;
	prevTotalUploaded = 0;
}

const TUS_ENDPOINT = "/api/upload/";

// The tus resume fingerprint (tus-<fileId>-<endpoint>, @uppy/tus
// getFingerprint) ignores upload metadata, so an entry orphaned by a
// mid-upload reload matches the same file re-dropped into a DIFFERENT
// directory; the server finalizes with the stored creation-time targetDir and
// the file silently lands in the old directory. @uppy/tus hard-overrides
// `fingerprint`, so scope resume candidates here instead: same localStorage
// scheme as tus-js-client's WebStorageUrlStorage (tus::<fingerprint>::<id>
// keys, entries carry `metadata`), but findUploadsByFingerprint only returns
// entries whose stored targetDir matches the current file's.
function readTusEntries(prefix: string): any[] {
	const results: any[] = [];
	try {
		for (let i = 0; i < localStorage.length; i++) {
			const key = localStorage.key(i);
			if (!key || !key.startsWith(prefix)) continue;
			try {
				const entry = JSON.parse(localStorage.getItem(key)!);
				entry.urlStorageKey = key;
				results.push(entry);
			} catch {
				// Malformed entry, skip it.
			}
		}
	} catch {
		// localStorage unavailable (sandboxed frame, private mode): no resume.
	}
	return results;
}

const dirScopedUrlStorage = {
	findAllUploads(): Promise<any[]> {
		return Promise.resolve(readTusEntries("tus::"));
	},
	findUploadsByFingerprint(fingerprint: string): Promise<any[]> {
		const suffix = `-${TUS_ENDPOINT}`;
		const fileId =
			fingerprint.startsWith("tus-") && fingerprint.endsWith(suffix)
				? fingerprint.slice(4, -suffix.length)
				: null;
		const meta = fileId
			? (instance?.getFile(fileId)?.meta as Record<string, unknown> | undefined)
			: undefined;
		const targetDir = typeof meta?.targetDir === "string" ? meta.targetDir : null;
		// Without a current targetDir to compare, offer nothing: a fresh upload
		// is always correct, a cross-directory resume never is.
		const entries = targetDir
			? readTusEntries(`tus::${fingerprint}::`).filter((e) => e?.metadata?.targetDir === targetDir)
			: [];
		return Promise.resolve(entries);
	},
	removeUpload(urlStorageKey: string): Promise<void> {
		try {
			localStorage.removeItem(urlStorageKey);
		} catch {
			// localStorage unavailable.
		}
		return Promise.resolve();
	},
	addUpload(fingerprint: string, upload: unknown): Promise<string> {
		const key = `tus::${fingerprint}::${Math.round(Math.random() * 1e12)}`;
		try {
			localStorage.setItem(key, JSON.stringify(upload));
		} catch {
			// Not persisted; removeUpload on this key is a harmless no-op.
		}
		return Promise.resolve(key);
	},
};

async function getUppy(): Promise<Uppy> {
	if (instance) return instance;

	const [{ default: UppyCore }, { default: Tus }, { emaFilter }] = await Promise.all([
		import("@uppy/core"),
		import("@uppy/tus"),
		import("@uppy/utils"),
	]);

	emaFilterFn = emaFilter;

	instance = new UppyCore({
		id: "onyx-uploader",
		autoProceed: false,
		allowMultipleUploadBatches: true,
	});

	instance.use(Tus, {
		endpoint: TUS_ENDPOINT,
		urlStorage: dirScopedUrlStorage,
		limit: 4, // per-IP cap is 8 (router.go); completions fire DELETE + next POST at once, so keep 2x limit within it
		retryDelays: [0, 1000, 3000, 5000],
		allowedMetaFields: true,
		removeFingerprintOnSuccess: true,
		headers: (): Record<string, string> => {
			const token = getCsrfToken();
			return token ? { "X-CSRF-Token": token } : {};
		},
	});

	instance.on("upload", () => {
		startFlushTimer();
	});

	instance.on("upload-progress", (file, progress) => {
		if (!file) return;
		rawProgress.set(file.id, progress.bytesUploaded);
		progressDirty = true;
	});

	instance.on("upload-success", (file) => {
		if (!file) return;
		rawProgress.delete(file.id);
		uploadState.markComplete(file.id);
		// Free Uppy's slot immediately and pull the next windowed file in; this
		// keeps Uppy's working set bounded instead of holding the whole folder.
		instance!.removeFile(file.id);
		pumpWindow();
	});

	instance.on("upload-error", (file, error) => {
		if (file) {
			// Drop stale sent-bytes, or complete's final flush re-flips this file
			// error→uploading and reconcileActive then marks it complete. A real
			// retry re-adds entries via fresh progress events.
			rawProgress.delete(file.id);
			uploadState.markError(file.id, uploadErrorMessage(error));
			// Never leave an errored file in Uppy: each pump's upload() retries
			// every errored file instance-wide, so a permanently failing file
			// would re-upload in full on every window refill. Grouped files
			// recover via re-drop → Merge; loose files are benched so the retry
			// button can re-add them.
			if (!uploadState.isGrouped(file.id)) {
				errBench.set(file.id, {
					name: file.name,
					type: file.type,
					data: file.data,
					meta: { ...file.meta },
				});
			}
			instance!.removeFile(file.id);
		}
		pumpWindow();
	});

	// Batch wrap-up: final progress flush, refill the window, stop the flush timer
	// once drained. (Completed files are removed eagerly in upload-success, not here.)
	instance.on("complete", () => {
		// Final flush to ensure UI shows latest progress
		for (const [id, bytesUploaded] of rawProgress) {
			uploadState.updateProgress(id, bytesUploaded);
		}

		// More windowed files still queued, keep feeding them.
		if (windowQueue.length > 0) {
			pumpWindow();
			return;
		}

		// Truly drained: every window emptied and nothing transferable remains.
		// Errored files are removed from Uppy on upload-error; the !error filter
		// is belt-and-braces so one stray failure can't leak the flush timer.
		if (instance!.getFiles().filter((f) => !f.error).length === 0) {
			stopFlushTimer();
			uploadState.updateSpeedAndEta(0, null);
			uploadState.reconcileActive(new Set<string>());
		}
	});

	return instance;
}

// Surface the server's sanitized "ERR_X: reason" message. tusd puts it in the
// response body and tus-js-client embeds it in its error dump; match whichever
// is present instead of showing the multi-line dump.
function uploadErrorMessage(error: unknown): string {
	const body = (error as any)?.originalResponse?.getBody?.();
	const message = (error as Error | undefined)?.message ?? "";
	const m = `${typeof body === "string" ? body : ""}\n${message}`.match(/ERR_[A-Z_]+: *([^\n]+)/);
	if (m) return m[1].trim();
	return message || "Upload failed";
}

export interface ConflictResolution {
	[filename: string]: "replace" | "keepBoth" | "skip";
}

export interface AddFilesOptions {
	// Per-file strategy keyed by relativePath; used for loose (non-folder) drops.
	resolutions?: ConflictResolution;
	// Per-top-level-folder strategy (e.g. "replace" to merge-overwrite into an
	// existing folder). Keyed by the original top-level folder name.
	strategyByTop?: Record<string, string>;
	// Rename a top-level folder before upload (keep-both). Maps original
	// top-level name → new unique name; rewrites every descendant's relativePath.
	folderRenames?: Record<string, string>;
}

let groupCounter = 0;

// Window so Uppy never materializes a whole giant folder at once. Grouped files
// wait in `windowQueue` and are fed to Uppy up to WINDOW_SIZE at a time, refilled
// as uploads finish. `enqueueAbort` lets cancel drain the queue mid-stream.
const WINDOW_SIZE = 200;
let windowQueue: { desc: any; group: string }[] = [];
let enqueueAbort = false;
// Errored loose files are removed from Uppy (see upload-error) but benched
// here so retryUpload can re-add them; keyed by Uppy file id.
const errBench = new Map<string, any>();

function topSegment(rel: string): string | null {
	const idx = rel.indexOf("/");
	return idx === -1 ? null : rel.slice(0, idx);
}

export async function addFiles(
	files: File[],
	targetDir: string,
	opts: AddFilesOptions = {},
) {
	const { resolutions, strategyByTop, folderRenames } = opts;
	const uppy = await getUppy();
	enqueueAbort = false;

	// Clear previous completed uploads before starting new batch
	uploadState.clearCompleted();

	// One group per top-level folder, created lazily as files reference it.
	// Keyed by the (renamed) top-level name so the panel shows the right label
	// and a single X cancels the whole folder.
	const groupIds = new Map<string, string>();
	const groupForTop = (top: string): string => {
		let gid = groupIds.get(top);
		if (!gid) {
			gid = `dir-${++groupCounter}-${top}`;
			groupIds.set(top, gid);
			uploadState.addGroup(gid, top, targetDir);
		}
		return gid;
	};

	// Build descriptors, splitting loose files (added immediately, tracked
	// per-file) from grouped files (windowed). Grouped totals are tallied here
	// so the panel can show the full count before any file enters Uppy.
	const looseDescs: any[] = [];
	const groupedDescs: { desc: any; group: string }[] = [];
	const groupTotals = new Map<string, { count: number; bytes: number }>();
	for (const file of files) {
		const rawRelPath = (file as any).webkitRelativePath || (file as any).relativePath || "";
		const origRel = rawRelPath || file.name;
		const origTop = topSegment(origRel);

		const resolution = origTop ? undefined : resolutions?.[origRel];
		if (resolution === "skip") continue;

		// Apply a folder rename (keep-both) to the relativePath we send + display.
		let relativePath = origRel;
		let group: string | undefined;
		if (origTop) {
			const newTop = folderRenames?.[origTop] ?? origTop;
			if (newTop !== origTop) relativePath = newTop + origRel.slice(origTop.length);
			group = groupForTop(newTop);
		}

		const conflictStrategy = origTop ? (strategyByTop?.[origTop] ?? "") : (resolution ?? "");

		const desc = {
			name: file.name,
			type: file.type,
			data: file,
			meta: {
				name: file.name,
				targetDir: targetDir || "/",
				relativePath,
				conflictStrategy,
			},
		};

		if (group) {
			const t = groupTotals.get(group) ?? { count: 0, bytes: 0 };
			t.count++;
			t.bytes += file.size;
			groupTotals.set(group, t);
			groupedDescs.push({ desc, group });
		} else {
			looseDescs.push(desc);
		}
	}

	// Register full group sizes up front, then stream files through the window.
	for (const [gid, t] of groupTotals) uploadState.setGroupTotals(gid, t.count, t.bytes);
	if (looseDescs.length > 0) addTracked(uppy, looseDescs.map((desc) => ({ desc })));
	windowQueue.push(...groupedDescs);
	pumpWindow();
}

// Add a batch of descriptors to Uppy and start per-file tracking by the id Uppy
// assigns. Grouped files go to trackFile (group totals already counted); loose
// files become per-file rows.
function addTracked(uppy: Uppy, batch: { desc: any; group?: string }[]) {
	const groupByData = new Map<unknown, string | undefined>();
	// Expected per-group adds; whatever Uppy rejects or trackFile declines to
	// bind is subtracted from the group's pre-registered totals so the group
	// can still reach complete.
	const missing = new Map<string, { count: number; bytes: number }>();
	for (const e of batch) {
		groupByData.set(e.desc.data, e.group);
		if (e.group) {
			const t = missing.get(e.group) ?? { count: 0, bytes: 0 };
			t.count++;
			t.bytes += e.desc.data?.size ?? 0;
			missing.set(e.group, t);
		}
	}

	// addFiles does NOT throw on duplicates/restrictions; it emits
	// 'restriction-failed' and silently skips them, so only genuinely-new files
	// arrive via 'file-added'. The try/catch guards the rare AggregateError.
	const added: { id: string; name: string; size: number; data: unknown }[] = [];
	const onFileAdded = (file: any) => {
		added.push({ id: file.id, name: file.name, size: file.size ?? 0, data: file.data });
	};
	// Uppy's file id covers name/type/relativePath/size/mtime but not targetDir, so
	// a file still in flight to another folder is rejected here too. Uppy reports
	// that only through its Informer, which this headless setup never renders.
	const blocked: { name: string; dir?: string }[] = [];
	const onRestrictionFailed = (file: any) => {
		if (!file || groupByData.get(file.data)) return;
		const meta = uppy.getFile(file.id)?.meta as Record<string, unknown> | undefined;
		const dir = typeof meta?.targetDir === "string" ? meta.targetDir : undefined;
		blocked.push({ name: file.name, dir: dir === file.meta?.targetDir ? undefined : dir });
	};
	uppy.on("file-added", onFileAdded);
	uppy.on("restriction-failed", onRestrictionFailed);
	try {
		uppy.addFiles(batch.map((e) => e.desc));
	} catch {
		// no-op
	}
	uppy.off("file-added", onFileAdded);
	uppy.off("restriction-failed", onRestrictionFailed);

	const looseBucket: { id: string; name: string; size: number }[] = [];
	for (const f of added) {
		const group = groupByData.get(f.data);
		if (group) {
			if (uploadState.trackFile(f.id, f.size, group)) {
				const t = missing.get(group)!;
				t.count--;
				t.bytes -= f.size;
			}
		} else {
			looseBucket.push({ id: f.id, name: f.name, size: f.size });
		}
	}
	if (looseBucket.length > 0) uploadState.addLooseFiles(looseBucket);

	let skipped = blocked.filter((b) => !b.dir).length;
	for (const [gid, t] of missing) {
		if (t.count > 0) {
			skipped += t.count;
			uploadState.adjustGroupShortfall(gid, t.count, t.bytes);
		}
	}
	if (skipped > 0) {
		toast.info(`Skipped ${skipped} file${skipped === 1 ? "" : "s"} already uploading`);
	}

	const elsewhere = blocked.filter((b): b is { name: string; dir: string } => !!b.dir);
	if (elsewhere.length > 0) {
		const dirs = new Set(elsewhere.map((b) => b.dir));
		const first = elsewhere[0].dir;
		const where = dirs.size === 1 ? (first === "/" ? "/" : `/${first}`) : "other folders";
		toast.info(
			elsewhere.length === 1
				? `"${elsewhere[0].name}" is already uploading to ${where}`
				: `${elsewhere.length} files are already uploading to ${where}`,
		);
	}
}

// Keep Uppy topped up to WINDOW_SIZE files from the grouped queue, then upload.
function pumpWindow() {
	if (!instance) return;
	if (enqueueAbort) {
		windowQueue = [];
		return;
	}
	if (windowQueue.length === 0) return;
	// Errored files are removed from Uppy on upload-error; filtering them here
	// is belt-and-braces so a stray failure can't shrink (or dead-stall) the pump.
	const slots = WINDOW_SIZE - instance.getFiles().filter((f) => !f.error).length;
	if (slots <= 0) return;
	const batch = windowQueue.splice(0, slots);
	addTracked(instance, batch);
	instance.upload().catch(() => {});
}

export async function startUpload() {
	if (!instance) return;
	return instance.upload();
}

export function cancelUpload(fileId: string) {
	if (!instance) return;
	errBench.delete(fileId);
	try {
		instance.removeFile(fileId);
	} catch {
		// Benched (errored) files are already out of Uppy.
	}
	uploadState.removeFile(fileId);
}

export function cancelGroup(groupId: string) {
	if (!instance) return;

	// Drop this group's not-yet-windowed files so the pump stops feeding them.
	windowQueue = windowQueue.filter((e) => e.group !== groupId);

	// Capture ids before removeGroup() clears the group's file records.
	for (const id of uploadState.groupFileIds(groupId)) {
		try {
			instance.removeFile(id);
		} catch {
			// File may already have been removed (completed and cleaned up)
		}
	}

	uploadState.removeGroup(groupId);

	// No server-side cleanup: tus is per-file atomic, so cancelling only drops
	// in-flight/queued files; already-completed files stay on disk. This matches
	// Drive/Dropbox/OneDrive (keep what finished, cancel the rest). The partial
	// folder is recoverable by re-dropping → Merge.
}

export function cancelAll() {
	if (!instance) return;

	// Stop the window pump and drop everything still queued (handles cancel
	// during the preparing/enqueue phase, not just active transfers).
	enqueueAbort = true;
	windowQueue = [];
	errBench.clear();

	instance.cancelAll();
	stopFlushTimer();
	uploadState.clear();

	// No server-side cleanup: already-completed files stay on disk; only in-flight
	// and queued transfers are dropped. Matches production uploaders; see cancelGroup.
}

export function retryUpload(fileId: string) {
	if (!instance) return;
	uploadState.retry(fileId);
	const benched = errBench.get(fileId);
	if (benched) {
		errBench.delete(fileId);
		addTracked(instance, [{ desc: benched }]);
		instance.upload().catch(() => {});
		return;
	}
	instance.retryUpload(fileId);
}
