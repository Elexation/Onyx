import Uppy from "@uppy/core";
import Tus from "@uppy/tus";
import { emaFilter } from "@uppy/utils";
import { uploadState } from "$lib/stores/upload.svelte.js";
import { deleteFiles } from "$lib/api/files.js";
import { getCsrfToken } from "$lib/api";

let instance: Uppy | null = null;

// Raw progress buffer — not reactive. Uppy events write here freely.
// The flush timer reads from here and batch-updates reactive uploadState.
const rawProgress = new Map<string, number>();
let progressDirty = false;

// Speed tracking (non-reactive)
let flushInterval: ReturnType<typeof setInterval> | null = null;
let prevTotalUploaded = 0;
let prevFlushTime = 0;
let smoothedSpeed = 0;
const SPEED_HALF_LIFE = 2000;
const FLUSH_INTERVAL_MS = 500;

function startFlushTimer() {
	if (flushInterval) return;
	prevFlushTime = Date.now();
	prevTotalUploaded = uploadState.totalBytesUploaded;
	smoothedSpeed = 0;

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
				: emaFilter(instantSpeed, smoothedSpeed, SPEED_HALF_LIFE, dt);
		}

		// Compute ETA
		const remaining = uploadState.totalBytes - totalUploaded;
		const eta = smoothedSpeed > 0 ? remaining / smoothedSpeed : null;

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
	prevTotalUploaded = 0;
}

export function getUppy(): Uppy {
	if (instance) return instance;

	instance = new Uppy({
		id: "onyx-uploader",
		autoProceed: false,
		allowMultipleUploadBatches: true,
	});

	instance.use(Tus, {
		endpoint: "/api/upload/",
		limit: 5,
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
		if (file) {
			rawProgress.delete(file.id);
			uploadState.markComplete(file.id);
		}
	});

	instance.on("upload-error", (file, error) => {
		if (file) uploadState.markError(file.id, error?.message ?? "Upload failed");
	});

	// Clear completed files from Uppy after batch finishes so duplicates can be re-uploaded
	instance.on("complete", (result) => {
		// Final flush to ensure UI shows latest progress
		for (const [id, bytesUploaded] of rawProgress) {
			uploadState.updateProgress(id, bytesUploaded);
		}

		for (const file of result.successful ?? []) {
			instance!.removeFile(file.id);
		}

		// Only stop the flush timer once all batches are drained — with
		// allowMultipleUploadBatches:true, a second batch may still be in flight.
		if (instance!.getFiles().length === 0) {
			stopFlushTimer();
			uploadState.updateSpeedAndEta(0, null);
		}

		// Clean up any orphaned uploadState items that are no longer tracked by Uppy
		const uppyIds = new Set(instance!.getFiles().map((f) => f.id));
		for (const item of uploadState.items) {
			if (!uppyIds.has(item.id) && item.status !== "complete") {
				uploadState.markComplete(item.id);
			}
		}

	});

	return instance;
}

export interface ConflictResolution {
	[filename: string]: "replace" | "keepBoth" | "skip";
}

let groupCounter = 0;

export async function addFiles(
	files: File[],
	targetDir: string,
	resolutions?: ConflictResolution,
) {
	const uppy = getUppy();
	const CHUNK_SIZE = 50;

	// Clear previous completed uploads before starting new batch
	uploadState.clearCompleted();

	// Detect directory upload: check if any file has a relativePath with /
	let groupId: string | undefined;
	for (const file of files) {
		const relPath = (file as any).webkitRelativePath || (file as any).relativePath || "";
		if (relPath && relPath.includes("/")) {
			const dirName = relPath.split("/")[0];
			groupId = `dir-${++groupCounter}-${dirName}`;
			uploadState.addGroup(groupId, dirName, targetDir);
			break;
		}
	}

	// Build file descriptors, filtering out skipped files.
	// Track per-file group membership so loose files mixed into a folder drop
	// aren't tagged with the directory's groupId (would orphan them on cancel).
	const descriptors: { desc: any; inGroup: boolean }[] = [];
	for (const file of files) {
		const rawRelPath = (file as any).webkitRelativePath || (file as any).relativePath || "";
		const relativePath = rawRelPath || file.name;
		const resolution = resolutions?.[relativePath];
		if (resolution === "skip") continue;

		descriptors.push({
			inGroup: !!(groupId && rawRelPath.includes("/")),
			desc: {
				name: file.name,
				type: file.type,
				data: file,
				meta: {
					name: file.name,
					targetDir: targetDir || "/",
					relativePath,
					conflictStrategy: resolution ?? "",
				},
			},
		});
	}

	// Process in chunks for browser responsiveness
	for (let i = 0; i < descriptors.length; i += CHUNK_SIZE) {
		const chunkEntries = descriptors.slice(i, i + CHUNK_SIZE);
		const inGroupByData = new Map<File, boolean>();
		for (const entry of chunkEntries) inGroupByData.set(entry.desc.data, entry.inGroup);
		const before = new Set(uppy.getFiles().map((f) => f.id));

		try {
			uppy.addFiles(chunkEntries.map((e) => e.desc));
		} catch {
			// Duplicates are handled as restriction errors (files still added).
			// AggregateError only for non-restriction errors — shouldn't happen.
		}

		const addedGrouped: { id: string; name: string; size: number }[] = [];
		const addedLoose: { id: string; name: string; size: number }[] = [];
		for (const f of uppy.getFiles()) {
			if (before.has(f.id)) continue;
			const desc = { id: f.id, name: f.name, size: f.size ?? 0 };
			const target = inGroupByData.get(f.data as File) ? addedGrouped : addedLoose;
			target.push(desc);
		}

		if (addedGrouped.length > 0) uploadState.addFiles(addedGrouped, groupId);
		if (addedLoose.length > 0) uploadState.addFiles(addedLoose);

		if (i + CHUNK_SIZE < descriptors.length) {
			await new Promise((resolve) => setTimeout(resolve, 0));
		}
	}
}

export function startUpload() {
	const uppy = getUppy();
	return uppy.upload();
}

export function cancelUpload(fileId: string) {
	const uppy = getUppy();
	uppy.removeFile(fileId);
	uploadState.removeFile(fileId);
}

export async function cancelGroup(groupId: string) {
	const uppy = getUppy();
	const meta = uploadState.groupMeta[groupId];
	const groupItems = uploadState.items.filter((i) => i.group === groupId);
	const hasActive = groupItems.some((i) => i.status !== "complete");

	for (const item of groupItems) {
		try {
			uppy.removeFile(item.id);
		} catch {
			// File may already have been removed (completed and cleaned up)
		}
	}

	uploadState.removeGroup(groupId);

	// Only delete server-side if the group didn't fully complete — otherwise we'd
	// wipe a successfully-uploaded directory.
	if (meta && hasActive) {
		const dirPath = meta.targetDir === "/"
			? `/${meta.name}`
			: `${meta.targetDir}/${meta.name}`;
		try {
			await deleteFiles([dirPath], true);
		} catch {
			// Directory may not exist yet if no files completed
		}
	}
}

export async function cancelAll() {
	const uppy = getUppy();

	// Skip fully-completed groups — their meta lingers until the next addFiles clears it,
	// and we must not delete successful uploads when cancelling a concurrent upload.
	const activeGroupIds = new Set(
		uploadState.items
			.filter((i) => i.status !== "complete")
			.map((i) => i.group)
			.filter((g): g is string => !!g),
	);
	const groupDirs: string[] = [];
	for (const [groupId, meta] of Object.entries(uploadState.groupMeta)) {
		if (!activeGroupIds.has(groupId)) continue;
		const dirPath = meta.targetDir === "/"
			? `/${meta.name}`
			: `${meta.targetDir}/${meta.name}`;
		groupDirs.push(dirPath);
	}

	uppy.cancelAll();
	stopFlushTimer();
	uploadState.clear();

	// Delete partially uploaded directories from server
	if (groupDirs.length > 0) {
		try {
			await deleteFiles(groupDirs, true);
		} catch {
			// Directories may not exist yet if no files completed
		}
	}
}

export function retryUpload(fileId: string) {
	const uppy = getUppy();
	uppy.retryUpload(fileId);
}
