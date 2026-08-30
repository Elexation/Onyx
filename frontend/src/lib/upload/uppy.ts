import type Uppy from "@uppy/core";
import { uploadState } from "$lib/stores/upload.svelte.js";
import { getCsrfToken } from "$lib/api";

let instance: Uppy | null = null;
let emaFilterFn: ((newValue: number, oldValue: number, halfLife: number, dt: number) => number) | null = null;

// Raw progress buffer — not reactive. Uppy events write here freely.
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

		// Compute ETA — withheld during warmup so the cold-seeded byte rate doesn't
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
		endpoint: "/api/upload/",
		limit: 20,
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
		// Free Uppy's slot immediately and pull the next windowed file in — this
		// keeps Uppy's working set bounded instead of holding the whole folder.
		instance!.removeFile(file.id);
		pumpWindow();
	});

	instance.on("upload-error", (file, error) => {
		if (file) {
			uploadState.markError(file.id, error?.message ?? "Upload failed");
			// Grouped files have no per-file retry UI (recovery is re-drop → Merge):
			// drop them from Uppy so they free their window slot and aren't silently
			// re-tried by the next pump's upload() (retry-all is instance-wide).
			// Loose files stay — the retry button needs them in Uppy.
			if (uploadState.isGrouped(file.id)) instance!.removeFile(file.id);
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

		// More windowed files still queued — keep feeding them.
		if (windowQueue.length > 0) {
			pumpWindow();
			return;
		}

		// Truly drained: every window emptied and nothing transferable remains.
		// Errored loose files stay in Uppy for the retry button, so count only
		// non-errored files — otherwise one failure leaks the flush timer forever.
		if (instance!.getFiles().filter((f) => !f.error).length === 0) {
			stopFlushTimer();
			uploadState.updateSpeedAndEta(0, null);
			uploadState.reconcileActive(new Set<string>());
		}
	});

	return instance;
}

export interface ConflictResolution {
	[filename: string]: "replace" | "keepBoth" | "skip";
}

export interface AddFilesOptions {
	// Per-file strategy keyed by relativePath — used for loose (non-folder) drops.
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
	for (const e of batch) groupByData.set(e.desc.data, e.group);

	// addFiles does NOT throw on duplicates/restrictions — it emits
	// 'restriction-failed' and silently skips them, so only genuinely-new files
	// arrive via 'file-added'. The try/catch guards the rare AggregateError.
	const added: { id: string; name: string; size: number; data: unknown }[] = [];
	const onFileAdded = (file: any) => {
		added.push({ id: file.id, name: file.name, size: file.size ?? 0, data: file.data });
	};
	uppy.on("file-added", onFileAdded);
	try {
		uppy.addFiles(batch.map((e) => e.desc));
	} catch {
		// no-op
	}
	uppy.off("file-added", onFileAdded);

	const looseBucket: { id: string; name: string; size: number }[] = [];
	for (const f of added) {
		const group = groupByData.get(f.data);
		if (group) uploadState.trackFile(f.id, f.size, group);
		else looseBucket.push({ id: f.id, name: f.name, size: f.size });
	}
	if (looseBucket.length > 0) uploadState.addLooseFiles(looseBucket);
}

// Keep Uppy topped up to WINDOW_SIZE files from the grouped queue, then upload.
function pumpWindow() {
	if (!instance) return;
	if (enqueueAbort) {
		windowQueue = [];
		return;
	}
	if (windowQueue.length === 0) return;
	// Errored files awaiting retry don't count against the window — otherwise
	// accumulated failures shrink (and can dead-stall) the pump.
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
	instance.removeFile(fileId);
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
	// in-flight/queued files — already-completed files stay on disk. This matches
	// Drive/Dropbox/OneDrive (keep what finished, cancel the rest). The partial
	// folder is recoverable by re-dropping → Merge.
}

export function cancelAll() {
	if (!instance) return;

	// Stop the window pump and drop everything still queued (handles cancel
	// during the preparing/enqueue phase, not just active transfers).
	enqueueAbort = true;
	windowQueue = [];

	instance.cancelAll();
	stopFlushTimer();
	uploadState.clear();

	// No server-side cleanup: already-completed files stay on disk; only in-flight
	// and queued transfers are dropped. Matches production uploaders — see cancelGroup.
}

export function retryUpload(fileId: string) {
	if (!instance) return;
	uploadState.retry(fileId);
	instance.retryUpload(fileId);
}
