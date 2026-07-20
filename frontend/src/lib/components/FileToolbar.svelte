<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import UploadButton from "$lib/components/UploadButton.svelte";
	import FolderPlusIcon from "@lucide/svelte/icons/folder-plus";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ClipboardPasteIcon from "@lucide/svelte/icons/clipboard-paste";
	import DownloadIcon from "@lucide/svelte/icons/download";
	import Share2Icon from "@lucide/svelte/icons/share-2";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import type { Snippet } from "svelte";

	let {
		onnewfolder,
		onrefresh,
		refreshing = false,
		ondelete,
		onpaste,
		ondownload,
		onshare,
		onupload,
		viewControls,
	}: {
		onnewfolder: () => void;
		onrefresh: () => void;
		refreshing?: boolean;
		ondelete: () => void;
		onpaste: () => void;
		ondownload: () => void;
		onshare: () => void;
		onupload: (files: File[]) => void;
		viewControls: Snippet;
	} = $props();
</script>

<div class="flex min-h-9 flex-wrap items-center gap-2">
	{#if selection.isActive}
		<!-- Selection-active: primary toolbar is replaced. Mobile shows icons only. -->
		<span
			class="inline-flex min-w-[5.5rem] items-center justify-center rounded-md bg-accent-brand-dim px-2.5 py-1 text-meta font-medium tabular-nums text-accent-brand"
		>
			{selection.count} selected
		</span>
		<Button variant="outline" size="sm" onclick={ondownload}>
			<DownloadIcon class="size-[15px]" strokeWidth={2} />
			<span class="max-md:hidden">Download</span>
		</Button>
		<Button
			variant="outline"
			size="sm"
			disabled={selection.count !== 1}
			onclick={onshare}
		>
			<Share2Icon class="size-[15px]" strokeWidth={2} />
			<span class="max-md:hidden">Share</span>
		</Button>
		<Button
			variant="outline"
			size="sm"
			class="text-destructive hover:bg-destructive/10 hover:text-destructive"
			onclick={ondelete}
		>
			<Trash2Icon class="size-[15px]" strokeWidth={2} />
			<span class="max-md:hidden">Delete</span>
		</Button>
	{:else}
		<div class="max-md:hidden">
			<UploadButton onfiles={onupload} />
		</div>
		<div class="max-md:hidden">
			<Button variant="outline" size="sm" onclick={onnewfolder}>
				<FolderPlusIcon class="size-[15px]" strokeWidth={2} />
				<span>New Folder</span>
			</Button>
		</div>
		{#if clipboard.hasItems}
			<Button variant="outline" size="sm" onclick={onpaste} class="max-md:px-2">
				<ClipboardPasteIcon class="size-[15px]" strokeWidth={2} />
				<span class="max-md:hidden">Paste</span>
			</Button>
		{/if}
	{/if}

	<Button variant="ghost" size="icon-sm" onclick={onrefresh} title="Refresh" aria-label="Refresh">
		<RefreshCwIcon class="size-[15px] {refreshing ? 'animate-spin' : ''}" strokeWidth={2} />
	</Button>

	<div class="ml-auto">
		{@render viewControls()}
	</div>
</div>
