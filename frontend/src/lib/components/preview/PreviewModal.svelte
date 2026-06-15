<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { getPreviewType, isPreviewTooLarge, getPreviewUrl } from "$lib/preview.js";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize } from "$lib/utils/format.js";
	import TextPreview from "./TextPreview.svelte";
	import MarkdownPreview from "./MarkdownPreview.svelte";
	import ImagePreview from "./ImagePreview.svelte";
	import VideoPreview from "./VideoPreview.svelte";
	import AudioPreview from "./AudioPreview.svelte";
	import PdfPreview from "./PdfPreview.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import XIcon from "@lucide/svelte/icons/x";
	import DownloadIcon from "@lucide/svelte/icons/download";

	let {
		file = $bindable(),
		items,
		onclose,
		url,
		downloadUrl,
	}: {
		file: FileInfo;
		items: FileInfo[];
		onclose: () => void;
		url?: string;
		downloadUrl?: string;
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
			>
				<DownloadIcon class="size-4" />
			</button>
			<button
				class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				onclick={onclose}
				title="Close"
			>
				<XIcon class="size-4" />
			</button>
		</div>
	</div>

	<div class="flex min-h-0 flex-1 flex-col" class:p-4={type !== "video"}>
		{#if tooLarge}
			<div class="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground">
				<p class="text-[15px]">File too large to preview (<span class="font-mono text-[13px]">{formatFileSize(file.size)}</span>)</p>
				<Button onclick={handleDownload}>
					<DownloadIcon class="mr-2 size-4" />
					Download
				</Button>
			</div>
		{:else if type === "text"}
			<TextPreview path={file.path} {url} />
		{:else if type === "markdown"}
			<MarkdownPreview path={file.path} {url} />
		{:else if type === "image"}
			<ImagePreview
				{file}
				siblings={imageSiblings}
				onnavigate={(f) => { file = f; }}
				{url}
			/>
		{:else if type === "video"}
			<VideoPreview {file} {onclose} {url} />
		{:else if type === "audio"}
			<AudioPreview path={file.path} {url} />
		{:else if type === "pdf"}
			<PdfPreview path={file.path} {url} />
		{/if}
	</div>
</div>
