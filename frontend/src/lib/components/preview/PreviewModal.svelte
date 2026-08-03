<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { getPreviewType, isPreviewTooLarge, getPreviewUrl } from "$lib/preview.js";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize } from "$lib/utils/format.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { viewport } from "$lib/stores/viewport.svelte.js";
	import XIcon from "@lucide/svelte/icons/x";
	import DownloadIcon from "@lucide/svelte/icons/download";

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

	let dialogEl = $state<HTMLDialogElement | null>(null);

	const type = $derived(getPreviewType(file));
	const tooLarge = $derived(isPreviewTooLarge(file));

	const imageSiblings = $derived(
		items.filter((i) => !i.isDir && getPreviewType(i) === "image")
	);

	function closeModal() {
		dialogEl?.close();
		onclose();
	}

	function handleBackdropClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (target.closest("[data-preview-content]")) return;
		closeModal();
	}

	function handleDownload() {
		const a = document.createElement("a");
		a.href = downloadUrl ?? getDownloadUrl(file.path);
		a.download = file.name;
		a.target = "_blank";
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
	}

	$effect(() => {
		dialogEl?.showModal();
		(document.activeElement as HTMLElement)?.blur();
	});
</script>

<dialog
	bind:this={dialogEl}
	aria-labelledby="preview-modal-title"
	class="fixed inset-0 z-50 m-0 flex h-full max-h-full w-full max-w-full flex-col overflow-hidden border-none bg-black/80 p-0"
	oncancel={(e) => { e.preventDefault(); closeModal(); }}
	onclick={handleBackdropClick}
>
	{#if type !== "pdf"}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="flex items-center justify-between border-b border-border bg-background/90 px-4 py-3 backdrop-blur-sm" onclick={(e) => e.stopPropagation()}>
			<h2 id="preview-modal-title" class="min-w-0 flex-1 truncate text-[15px] font-medium" title={file.name}>{file.name}</h2>
			<div class="flex items-center gap-1">
				<Button variant="ghost" size="icon-touch" class="text-muted-foreground"
					onclick={handleDownload}
					title="Download"
					aria-label="Download"
				>
					<DownloadIcon class="size-4" />
				</Button>
				<Button variant="ghost" size="icon-touch" class="text-muted-foreground"
					onclick={closeModal}
					title="Close"
					aria-label="Close preview"
				>
					<XIcon class="size-4" />
				</Button>
			</div>
		</div>
	{/if}

	<div class="flex min-h-0 flex-1 flex-col" class:p-4={type !== "video" && type !== "pdf"}>
		{#snippet previewLoading()}
			<div class="flex flex-1 items-center justify-center text-muted-foreground">
				<p class="text-[15px]">Loading…</p>
			</div>
		{/snippet}
		{#snippet previewError()}
			<div class="flex flex-1 items-center justify-center text-destructive">
				<p class="text-[15px]">Failed to load preview</p>
			</div>
		{/snippet}

		{#if tooLarge}
			<div class="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground">
				<p class="text-[15px]">File too large to preview (<span class="font-mono text-meta">{formatFileSize(file.size)}</span>)</p>
				<Button onclick={handleDownload}>
					<DownloadIcon class="mr-2 size-4" />
					Download
				</Button>
			</div>
		{:else if type === "text"}
			{#await previewLoaders.text()}
				{@render previewLoading()}
			{:then mod}
				{@const TextPreview = mod.default}
				<TextPreview path={file.path} {url} />
			{:catch}
				{@render previewError()}
			{/await}
		{:else if type === "markdown"}
			{#await previewLoaders.markdown()}
				{@render previewLoading()}
			{:then mod}
				{@const MarkdownPreview = mod.default}
				<MarkdownPreview path={file.path} {url} />
			{:catch}
				{@render previewError()}
			{/await}
		{:else if type === "image"}
			{#await previewLoaders.image()}
				{@render previewLoading()}
			{:then mod}
				{@const ImagePreview = mod.default}
				<ImagePreview
					{file}
					siblings={imageSiblings}
					onnavigate={(f) => { file = f; }}
					{url}
				/>
			{:catch}
				{@render previewError()}
			{/await}
		{:else if type === "video"}
			{#await previewLoaders.video()}
				{@render previewLoading()}
			{:then mod}
				{@const VideoPreview = mod.default}
				<VideoPreview {file} onclose={closeModal} {url} {streamBase} />
			{:catch}
				{@render previewError()}
			{/await}
		{:else if type === "audio"}
			{#await previewLoaders.audio()}
				{@render previewLoading()}
			{:then mod}
				{@const AudioPreview = mod.default}
				<AudioPreview path={file.path} {url} />
			{:catch}
				{@render previewError()}
			{/await}
		{:else if type === "pdf"}
			{#await previewLoaders.pdf()}
				{@render previewLoading()}
			{:then mod}
				{@const PdfPreview = mod.default}
				<PdfPreview path={file.path} {url} name={file.name} ondownload={handleDownload} onclose={closeModal} />
			{:catch}
				{@render previewError()}
			{/await}
		{/if}
	</div>
</dialog>

<style>
	dialog::backdrop {
		background: transparent;
	}
</style>
