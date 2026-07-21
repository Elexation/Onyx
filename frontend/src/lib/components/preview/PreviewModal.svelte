<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { getPreviewType, isPreviewTooLarge, getPreviewUrl } from "$lib/preview.js";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize } from "$lib/utils/format.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import XIcon from "@lucide/svelte/icons/x";
	import DownloadIcon from "@lucide/svelte/icons/download";

	// Preview components are dynamically imported so heavy deps
	// (shiki, pdfjs-dist, marked, dompurify) stay out of the
	// /files route chunk until first preview-open of that type.
	const previewLoaders = {
		text: () => import("./TextPreview.svelte"),
		markdown: () => import("./MarkdownPreview.svelte"),
		image: () => import("./ImagePreview.svelte"),
		video: () => import("./VideoPreview.svelte"),
		audio: () => import("./AudioPreview.svelte"),
		pdf: () => import("./PdfPreview.svelte"),
	};

	let {
		file = $bindable(),
		items,
		onclose,
		url,
		downloadUrl,
		streamBase,
	}: {
		file: FileInfo;
		items: FileInfo[];
		onclose: () => void;
		url?: string;
		downloadUrl?: string;
		streamBase?: string;
	} = $props();

	const type = $derived(getPreviewType(file));
	const tooLarge = $derived(isPreviewTooLarge(file));

	const imageSiblings = $derived(
		items.filter((i) => !i.isDir && getPreviewType(i) === "image")
	);

	function handleBackdropClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (target.closest("[data-preview-content]")) return;
		onclose();
	}

	function handleDownload() {
		const a = document.createElement("a");
		a.href = downloadUrl ?? getDownloadUrl(file.path);
		a.download = file.name;
		a.click();
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	class="fixed inset-0 z-50 flex flex-col bg-black/80"
	onclick={handleBackdropClick}
>
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="flex items-center justify-between border-b border-border bg-background/90 px-4 py-3 backdrop-blur-sm" onclick={(e) => e.stopPropagation()}>
		<h2 class="min-w-0 flex-1 truncate text-[15px] font-medium">{file.name}</h2>
		<div class="flex items-center gap-1">
			<button
				class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				onclick={handleDownload}
				title="Download"
				aria-label="Download"
			>
				<DownloadIcon class="size-4" />
			</button>
			<button
				class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				onclick={onclose}
				title="Close"
				aria-label="Close preview"
			>
				<XIcon class="size-4" />
			</button>
		</div>
	</div>

	<div class="flex min-h-0 flex-1 flex-col" class:p-4={type !== "video"}>
		{#if tooLarge}
			<div class="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground">
				<p class="text-[15px]">File too large to preview (<span class="font-mono text-meta">{formatFileSize(file.size)}</span>)</p>
				<Button onclick={handleDownload}>
					<DownloadIcon class="mr-2 size-4" />
					Download
				</Button>
			</div>
		{:else if type === "text"}
			{#await previewLoaders.text() then mod}
				{@const TextPreview = mod.default}
				<TextPreview path={file.path} {url} />
			{/await}
		{:else if type === "markdown"}
			{#await previewLoaders.markdown() then mod}
				{@const MarkdownPreview = mod.default}
				<MarkdownPreview path={file.path} {url} />
			{/await}
		{:else if type === "image"}
			{#await previewLoaders.image() then mod}
				{@const ImagePreview = mod.default}
				<ImagePreview
					{file}
					siblings={imageSiblings}
					onnavigate={(f) => { file = f; }}
					{url}
				/>
			{/await}
		{:else if type === "video"}
			{#await previewLoaders.video() then mod}
				{@const VideoPreview = mod.default}
				<VideoPreview {file} {onclose} {url} {streamBase} />
			{/await}
		{:else if type === "audio"}
			{#await previewLoaders.audio() then mod}
				{@const AudioPreview = mod.default}
				<AudioPreview path={file.path} {url} />
			{/await}
		{:else if type === "pdf"}
			{#await previewLoaders.pdf() then mod}
				{@const PdfPreview = mod.default}
				<PdfPreview path={file.path} {url} />
			{/await}
		{/if}
	</div>
</div>
