<script lang="ts">
	import { getDroppedFiles } from "@uppy/utils";
	import { uploadState } from "$lib/stores/upload.svelte.js";
	import { toast } from "svelte-sonner";
	import UploadIcon from "@lucide/svelte/icons/upload";
	import type { Snippet } from "svelte";

	let {
		currentDir,
		onupload,
		children,
	}: {
		currentDir: string;
		onupload: (files: File[], emptyDirs: string[]) => void | Promise<void>;
		children: Snippet;
	} = $props();

	let dragging = $state(false);
	let dragCounter = 0;
	// Above this file count, skip empty-folder detection — the extra full tree
	// walk doubles an already-heavy enumeration, and huge folders rarely depend
	// on preserving empty subdirectories.
	const EMPTY_DIR_SCAN_LIMIT = 5000;

	function isFileDrag(event: DragEvent) {
		return (
			event.dataTransfer?.types?.includes("Files") &&
			!document.body.classList.contains("onyx-internal-drag")
		);
	}

	function handleDragEnter(event: DragEvent) {
		if (!isFileDrag(event)) return;
		event.preventDefault();
		dragCounter++;
		dragging = true;
	}

	function handleDragOver(event: DragEvent) {
		if (!isFileDrag(event)) return;
		event.preventDefault();
		event.dataTransfer!.dropEffect = "copy";
	}

	function handleDragLeave() {
		dragCounter--;
		if (dragCounter <= 0) {
			dragCounter = 0;
			dragging = false;
		}
	}

	// Read every entry under a directory; readEntries returns in batches and
	// must be called until it yields an empty array.
	function readAllEntries(reader: any): Promise<any[]> {
		return new Promise((resolve) => {
			const acc: any[] = [];
			const next = () =>
				reader.readEntries(
					(batch: any[]) => {
						if (batch.length) {
							acc.push(...batch);
							next();
						} else {
							resolve(acc);
						}
					},
					() => resolve(acc),
				);
			next();
		});
	}

	// Collect every directory path (relative to the drop root, no leading slash —
	// matching Uppy's file.relativePath format) so we can recreate empty ones,
	// which getDroppedFiles discards.
	async function collectDirPaths(entries: any[]): Promise<string[]> {
		const dirs: string[] = [];
		const walk = async (entry: any, prefix: string) => {
			if (!entry || !entry.isDirectory) return;
			const full = prefix ? `${prefix}/${entry.name}` : entry.name;
			dirs.push(full);
			const children = await readAllEntries(entry.createReader());
			for (const child of children) await walk(child, full);
		};
		for (const e of entries) await walk(e, "");
		return dirs;
	}

	function computeEmptyDirs(allDirs: string[], files: File[]): string[] {
		// A dir is "occupied" if any file lives somewhere beneath it — those get
		// created implicitly by the file writes. The rest are the empty dirs.
		const occupied = new Set<string>();
		for (const f of files) {
			const rel = (f as any).relativePath || (f as any).webkitRelativePath || "";
			if (!rel.includes("/")) continue;
			const segs = rel.split("/");
			segs.pop(); // drop the filename
			for (let i = 1; i <= segs.length; i++) occupied.add(segs.slice(0, i).join("/"));
		}
		return allDirs.filter((d) => !occupied.has(d));
	}

	async function handleDrop(event: DragEvent) {
		if (!isFileDrag(event)) return;
		event.preventDefault();
		dragCounter = 0;
		dragging = false;

		if (uploadState.scanning || uploadState.preparing) {
			toast.info("Hang on — still preparing the previous drop");
			return;
		}

		const dt = event.dataTransfer!;
		// Kick off file enumeration first (it captures its own entries
		// synchronously), then grab our own entry handles for empty-dir
		// detection. Both reads must happen before any await, while the
		// DataTransfer is still live. Empty-dir detection is best-effort.
		const filesPromise = getDroppedFiles(dt);
		let rootEntries: any[] = [];
		try {
			rootEntries = Array.from(dt.items, (i) => (i as any).webkitGetAsEntry?.()).filter(Boolean);
		} catch {
			rootEntries = [];
		}

		uploadState.scanning = true;
		let files: File[] = [];
		let emptyDirs: string[] = [];
		try {
			files = await filesPromise;
			// Cancelled mid-scan (the preparing X clears scanning) — bail.
			if (!uploadState.scanning) return;

			if (files.length <= EMPTY_DIR_SCAN_LIMIT) {
				try {
					const allDirs = await collectDirPaths(rootEntries);
					emptyDirs = computeEmptyDirs(allDirs, files);
				} catch {
					emptyDirs = [];
				}
			}
			if (!uploadState.scanning) return;
		} catch {
			toast.error("Couldn't read the dropped items");
			return;
		} finally {
			uploadState.scanning = false;
		}

		// Outside the enumeration try — an upload failure here shouldn't surface
		// as "couldn't read the dropped items".
		if (files.length > 0 || emptyDirs.length > 0) {
			try {
				await onupload(files, emptyDirs);
			} catch {
				toast.error("Upload failed to start");
			}
		}
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="relative flex min-h-0 flex-1 flex-col"
	ondragenter={handleDragEnter}
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
>
	{@render children()}

	{#if dragging}
		<div
			class="pointer-events-none absolute inset-0 z-50 flex items-center justify-center rounded-xl border-2 border-dashed border-accent-brand bg-accent-brand-dim backdrop-blur-sm"
		>
			<div class="flex flex-col items-center gap-2 text-accent-brand">
				<UploadIcon class="size-10" strokeWidth={1.5} />
				<span class="text-sm font-medium">
					Drop files to upload to {currentDir || "/"}
				</span>
			</div>
		</div>
	{/if}
</div>
