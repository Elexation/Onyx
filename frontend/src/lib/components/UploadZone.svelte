<script lang="ts">
	import { onMount } from "svelte";
	import DropTarget from "@uppy/drop-target";
	import UploadIcon from "@lucide/svelte/icons/upload";
	import { getUppy, startUpload } from "$lib/upload/uppy.js";
	import { checkConflicts } from "$lib/api/upload.js";
	import type { Snippet } from "svelte";

	let {
		currentDir,
		onconflicts,
		children,
	}: {
		currentDir: string;
		onconflicts: (fileIds: string[], conflicts: string[]) => void;
		children: Snippet;
	} = $props();

	let container: HTMLDivElement;
	let dragging = $state(false);

	onMount(() => {
		const uppy = getUppy();

		uppy.use(DropTarget, {
			target: container,
			onDragOver: () => {
				dragging = true;
			},
			onDragLeave: () => {
				dragging = false;
			},
			onDrop: () => {
				dragging = false;
			},
		});

		// When DropTarget adds files, set metadata and run conflict check
		const handler = async (files: any[]) => {
			// Only process files that lack our targetDir metadata (i.e. from drops)
			const dropped = files.filter((f) => !f.meta.targetDir);
			if (dropped.length === 0) return;

			const dir = currentDir || "/";
			const fileIds: string[] = [];
			const relativePaths: string[] = [];

			for (const file of dropped) {
				const rp = file.meta.relativePath || file.name;
				uppy.setFileMeta(file.id, {
					targetDir: dir,
					name: file.name,
					relativePath: rp,
				});
				fileIds.push(file.id);
				relativePaths.push(rp);
			}

			try {
				const { conflicts } = await checkConflicts(dir, relativePaths);
				if (conflicts.length > 0) {
					onconflicts(fileIds, conflicts);
				} else {
					startUpload();
				}
			} catch {
				startUpload();
			}
		};

		uppy.on("files-added", handler);

		return () => {
			uppy.off("files-added", handler);
			const plugin = uppy.getPlugin("DropTarget");
			if (plugin) uppy.removePlugin(plugin);
		};
	});
</script>

<div bind:this={container} class="relative flex min-h-0 flex-1 flex-col">
	{@render children()}

	{#if dragging}
		<div
			class="pointer-events-none absolute inset-0 z-50 flex items-center justify-center rounded-lg border-2 border-dashed border-primary bg-primary/10 backdrop-blur-sm"
		>
			<div class="flex flex-col items-center gap-2 text-primary">
				<UploadIcon class="size-10" />
				<span class="text-sm font-medium">
					Drop files to upload to {currentDir || "/"}
				</span>
			</div>
		</div>
	{/if}
</div>
