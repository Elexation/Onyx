<script lang="ts">
	import { getDroppedFiles } from "@uppy/utils";
	import UploadIcon from "@lucide/svelte/icons/upload";
	import type { Snippet } from "svelte";

	let {
		currentDir,
		onupload,
		children,
	}: {
		currentDir: string;
		onupload: (files: File[]) => void;
		children: Snippet;
	} = $props();

	let dragging = $state(false);
	let dragCounter = 0;

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

	async function handleDrop(event: DragEvent) {
		if (!isFileDrag(event)) return;
		event.preventDefault();
		dragCounter = 0;
		dragging = false;

		const files = await getDroppedFiles(event.dataTransfer!);
		if (files.length > 0) {
			onupload(files);
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
