<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import type { FileInfo } from "$lib/types";

	let {
		file,
		siblings,
		onnavigate,
		url,
	}: {
		file: FileInfo;
		siblings: FileInfo[];
		onnavigate: (file: FileInfo) => void;
		url?: string;
	} = $props();

	let failed = $state(false);

	$effect(() => {
		file.path;
		failed = false;
	});

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
		{#if failed}
			<p class="text-[15px] text-muted-foreground">Unable to load image</p>
		{:else}
			<img
				src={url ?? getPreviewUrl(file.path)}
				alt={file.name}
				class="max-h-full max-w-full object-contain"
				onerror={() => failed = true}
				data-preview-content
			/>
		{/if}
	</div>

	{#if siblings.length > 1}
		<div class="flex items-center gap-3 text-muted-foreground" data-preview-content>
			<button
				class="rounded-md px-2.5 py-1 text-[13px] transition-colors hover:bg-muted hover:text-foreground disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
				disabled={!hasPrev}
				onclick={prev}
			>
				&larr; Prev
			</button>
			<span class="font-mono text-[13px]">{currentIndex + 1} / {siblings.length}</span>
			<button
				class="rounded-md px-2.5 py-1 text-[13px] transition-colors hover:bg-muted hover:text-foreground disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
				disabled={!hasNext}
				onclick={next}
			>
				Next &rarr;
			</button>
		</div>
	{/if}
</div>
