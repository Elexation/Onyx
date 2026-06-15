<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import UploadButton from "$lib/components/UploadButton.svelte";
	import FolderPlusIcon from "@lucide/svelte/icons/folder-plus";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import ScissorsIcon from "@lucide/svelte/icons/scissors";
	import ClipboardPasteIcon from "@lucide/svelte/icons/clipboard-paste";
	import DownloadIcon from "@lucide/svelte/icons/download";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import type { Snippet } from "svelte";

	let {
		onnewfolder,
		onrefresh,
		ondelete,
		onpaste,
		oncopy,
		oncut,
		ondownload,
		onupload,
		viewControls,
	}: {
		onnewfolder: () => void;
		onrefresh: () => void;
		ondelete: () => void;
		onpaste: () => void;
		oncopy: () => void;
		oncut: () => void;
		ondownload: () => void;
		onupload: (files: File[]) => void;
		viewControls: Snippet;
	} = $props();
</script>

<div class="flex min-h-9 flex-wrap items-center gap-2">
	<!-- Primary Upload CTA: hidden on mobile (replaced by FAB) -->
	<div class="max-md:hidden">
		<UploadButton onfiles={onupload} />
	</div>

	<!-- New Folder: hidden on mobile (moved into FAB menu) -->
	<div class="max-md:hidden">
		<Button variant="outline" size="sm" onclick={onnewfolder}>
			<FolderPlusIcon class="size-[15px]" strokeWidth={2} />
			<span>New Folder</span>
		</Button>
	</div>
	<Button variant="ghost" size="icon-sm" onclick={onrefresh} title="Refresh" aria-label="Refresh">
		<RefreshCwIcon class="size-[15px]" strokeWidth={2} />
	</Button>

	{#if selection.isActive}
		<!-- Desktop selection chip (hidden on mobile — replaced by floating selection bar) -->
		<div
			class="inline-flex items-center gap-1 rounded-lg border border-border-2 bg-card py-1 pr-1 pl-3 max-md:hidden"
		>
			<span class="mr-1 text-[13px] text-muted-foreground">{selection.count} selected</span>
			<Button variant="ghost" size="icon-xs" class="size-7" onclick={oncopy} title="Copy">
				<CopyIcon class="size-[15px]" strokeWidth={2} />
			</Button>
			<Button variant="ghost" size="icon-xs" class="size-7" onclick={oncut} title="Cut">
				<ScissorsIcon class="size-[15px]" strokeWidth={2} />
			</Button>
			{#if clipboard.hasItems}
				<Button variant="ghost" size="icon-xs" class="size-7" onclick={onpaste} title="Paste">
					<ClipboardPasteIcon class="size-[15px]" strokeWidth={2} />
				</Button>
			{/if}
			<Button variant="ghost" size="icon-xs" class="size-7" onclick={ondownload} title="Download">
				<DownloadIcon class="size-[15px]" strokeWidth={2} />
			</Button>
			<Button
				variant="ghost"
				size="icon-xs"
				class="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
				onclick={ondelete}
				title="Delete"
			>
				<Trash2Icon class="size-[15px]" strokeWidth={2} />
			</Button>
		</div>
	{:else if clipboard.hasItems}
		<Button variant="outline" size="sm" onclick={onpaste} class="max-md:px-2">
			<ClipboardPasteIcon class="size-[15px]" strokeWidth={2} />
			<span class="max-md:hidden">Paste</span>
		</Button>
	{/if}

	<div class="ml-auto">
		{@render viewControls()}
	</div>
</div>
