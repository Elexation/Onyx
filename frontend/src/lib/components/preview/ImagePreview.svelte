<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import type { FileInfo } from "$lib/types";

	let {
		file,
		siblings,
		onnavigate,
	}: {
		file: FileInfo;
		siblings: FileInfo[];
		onnavigate: (file: FileInfo) => void;
	} = $props();

	const currentIndex = $derived(siblings.findIndex((s) => s.path === file.path));

	const hasPrev = $derived(currentIndex > 0);
	const hasNext = $derived(currentIndex < siblings.length - 1);

	function prev() {
		if (hasPrev) onnavigate(siblings[currentIndex - 1]);
	}

	function next() {
		if (hasNext) onnavigate(siblings[currentIndex + 1]);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "ArrowLeft") {
			e.preventDefault();
			prev();
		} else if (e.key === "ArrowRight") {
			e.preventDefault();
			next();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-1 flex-col items-center justify-center gap-4 overflow-hidden">
	<div class="flex max-h-full max-w-full flex-1 items-center justify-center overflow-hidden">
		<img
			src={getPreviewUrl(file.path)}
			alt={file.name}
			class="h-full w-full object-contain"
		/>
	</div>

	{#if siblings.length > 1}
		<div class="flex items-center gap-3 text-sm text-muted-foreground">
			<button
				class="rounded px-2 py-1 transition-colors hover:bg-accent hover:text-foreground disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
				disabled={!hasPrev}
				onclick={prev}
			>
				&larr; Prev
			</button>
			<span>{currentIndex + 1} / {siblings.length}</span>
			<button
				class="rounded px-2 py-1 transition-colors hover:bg-accent hover:text-foreground disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
				disabled={!hasNext}
				onclick={next}
			>
				Next &rarr;
			</button>
		</div>
	{/if}
</div>
