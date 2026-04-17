import Uppy from "@uppy/core";
import Tus from "@uppy/tus";
import GoldenRetriever from "@uppy/golden-retriever";
import { uploadState } from "$lib/stores/upload.svelte.js";

let instance: Uppy | null = null;

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
	});

	instance.use(GoldenRetriever, {
		expires: 24 * 60 * 60 * 1000,
	});

	instance.on("file-added", (file) => {
		uploadState.addFile(file.id, file.name ?? "unknown", file.size ?? 0);
	});

	instance.on("upload-progress", (file, progress) => {
		if (!file) return;
		const pct = progress.bytesTotal
			? Math.round((progress.bytesUploaded / progress.bytesTotal) * 100)
			: 0;
		uploadState.updateProgress(file.id, pct);
	});

	instance.on("upload-success", (file) => {
		if (file) uploadState.markComplete(file.id);
	});

	instance.on("upload-error", (file, error) => {
		if (file) uploadState.markError(file.id, error?.message ?? "Upload failed");
	});

	// Clear completed files from Uppy after batch finishes so duplicates can be re-uploaded
	instance.on("complete", (result) => {
		for (const file of result.successful ?? []) {
			instance!.removeFile(file.id);
		}
	});

	return instance;
}

export interface ConflictResolution {
	[filename: string]: "replace" | "keepBoth" | "skip";
}

export function addFiles(
	files: File[],
	targetDir: string,
	resolutions?: ConflictResolution,
) {
	const uppy = getUppy();

	for (const file of files) {
		const relativePath = (file as any).webkitRelativePath || file.name;
		const resolution = resolutions?.[relativePath];

		if (resolution === "skip") continue;

		const fileOpts = {
			name: file.name,
			type: file.type,
			data: file,
			meta: {
				name: file.name,
				targetDir: targetDir || "/",
				relativePath,
				conflictStrategy: resolution ?? "",
			},
		};

		try {
			uppy.addFile(fileOpts);
		} catch {
			// Duplicate — remove existing and re-add
			const existing = uppy.getFiles().find((f) => f.name === file.name);
			if (existing) uppy.removeFile(existing.id);
			try {
				uppy.addFile(fileOpts);
			} catch {
				// give up
			}
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

export function cancelAll() {
	const uppy = getUppy();
	uppy.cancelAll();
	uploadState.clear();
}

export function retryUpload(fileId: string) {
	const uppy = getUppy();
	uppy.retryUpload(fileId);
}
