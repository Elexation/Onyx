<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { getPreviewType, isPreviewTooLarge, getPreviewUrl } from "$lib/preview.js";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize } from "$lib/utils/format.js";
	import TextPreview from "./TextPreview.svelte";
	import MarkdownPreview from "./MarkdownPreview.svelte";
	import ImagePreview from "./ImagePreview.svelte";
	import XIcon from "@lucide/svelte/icons/x";
	import DownloadIcon from "@lucide/svelte/icons/download";

	let {
		file = $bindable(),
		items,
		onclose,
	}: {
		file: FileInfo;
		items: FileInfo[];
		onclose: () => void;
	} = $props();

	const type = $derived(getPreviewType(file));
	const tooLarge = $derived(isPreviewTooLarge(file));

	const imageSiblings = $derived(
		items.filter((i) => !i.isDir && getPreviewType(i) === "image")
	);

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) onclose();
	}

	function handleDownload() {
		const a = document.createElement("a");
		a.href = getDownloadUrl(file.path);
		a.download = file.name;
		a.click();
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	class="fixed inset-0 z-50 flex flex-col bg-black/80"
	onclick={handleBackdropClick}
>
	<div class="flex items-center justify-between border-b border-border/50 bg-background/90 px-4 py-3 backdrop-blur-sm">
		<h2 class="min-w-0 flex-1 truncate text-sm font-medium">{file.name}</h2>
		<div class="flex items-center gap-2">
			<button
				class="rounded p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
				onclick={handleDownload}
				title="Download"
			>
				<DownloadIcon class="size-4" />
			</button>
			<button
				class="rounded p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
				onclick={onclose}
				title="Close"
			>
				<XIcon class="size-4" />
			</button>
		</div>
	</div>

	<div class="flex min-h-0 flex-1 flex-col p-4" onclick={(e) => e.stopPropagation()}>
		{#if tooLarge}
			<div class="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground">
				<p class="text-sm">File too large to preview ({formatFileSize(file.size)})</p>
				<button
					class="rounded-md bg-accent px-4 py-2 text-sm text-foreground transition-colors hover:bg-accent/80"
					onclick={handleDownload}
				>
					Download
				</button>
			</div>
		{:else if type === "text"}
			<TextPreview path={file.path} />
		{:else if type === "markdown"}
			<MarkdownPreview path={file.path} />
		{:else if type === "image"}
			<ImagePreview
				{file}
				siblings={imageSiblings}
				onnavigate={(f) => { file = f; }}
			/>
		{/if}
	</div>
</div>
