<script lang="ts">
	import * as ContextMenu from "$lib/components/ui/context-menu/index.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { versioningEnabled } from "$lib/stores/versioningEnabled.svelte.js";
	import { viewport } from "$lib/stores/viewport.svelte.js";
	import { getDownloadUrl } from "$lib/api/files.js";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import ScissorsIcon from "@lucide/svelte/icons/scissors";
	import ClipboardPasteIcon from "@lucide/svelte/icons/clipboard-paste";
	import FolderInputIcon from "@lucide/svelte/icons/folder-input";
	import FolderOutputIcon from "@lucide/svelte/icons/folder-output";
	import Share2Icon from "@lucide/svelte/icons/share-2";
	import DownloadIcon from "@lucide/svelte/icons/download";
	import HistoryIcon from "@lucide/svelte/icons/history";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import type { Snippet } from "svelte";

	let {
		item,
		onopen,
		onrename,
		ondelete,
		onmoveto,
		oncopyto,
		onpaste,
		onversions,
		onshare,
		trigger,
	}: {
		item: { name: string; path: string; isDir: boolean } | null;
		onopen: () => void;
		onrename: () => void;
		ondelete: () => void;
		onmoveto: () => void;
		oncopyto: () => void;
		onpaste: () => void;
		onversions: () => void;
		onshare: () => void;
		trigger: Snippet<[Record<string, any>]>;
	} = $props();

	function handleCopy() {
		const paths = selection.has(item?.path ?? "") && selection.count > 1
			? [...selection.items]
			: item ? [item.path] : [];
		clipboard.copy(paths);
	}

	function handleCut() {
		const paths = selection.has(item?.path ?? "") && selection.count > 1
			? [...selection.items]
			: item ? [item.path] : [];
		clipboard.cut(paths);
	}

	function handleDownload() {
		if (!item || item.isDir) return;
		const a = document.createElement("a");
		a.href = getDownloadUrl(item.path);
		a.download = item.name;
		a.click();
	}
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger disabled={viewport.isMobile}>
		{#snippet child({ props })}
			{@render trigger(props)}
		{/snippet}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52">
		{#if item}
			<ContextMenu.Item onclick={onopen}>
				<EyeIcon />
				Open
				<ContextMenu.Shortcut>Enter</ContextMenu.Shortcut>
			</ContextMenu.Item>
			<ContextMenu.Separator />
			{#if selection.count <= 1}
				<ContextMenu.Item onclick={onrename}>
					<PencilIcon />
					Rename
					<ContextMenu.Shortcut>F2</ContextMenu.Shortcut>
				</ContextMenu.Item>
			{/if}
			<ContextMenu.Item onclick={handleCopy}>
				<CopyIcon />
				Copy
				<ContextMenu.Shortcut>Ctrl+C</ContextMenu.Shortcut>
			</ContextMenu.Item>
			<ContextMenu.Item onclick={handleCut}>
				<ScissorsIcon />
				Cut
				<ContextMenu.Shortcut>Ctrl+X</ContextMenu.Shortcut>
			</ContextMenu.Item>
			{#if clipboard.hasItems}
				<ContextMenu.Separator />
			{/if}
		{/if}
		{#if clipboard.hasItems}
			<ContextMenu.Item onclick={onpaste}>
				<ClipboardPasteIcon />
				Paste
				<ContextMenu.Shortcut>Ctrl+V</ContextMenu.Shortcut>
			</ContextMenu.Item>
		{/if}
		{#if item}
			<ContextMenu.Separator />
			<ContextMenu.Item onclick={onmoveto}>
				<FolderInputIcon />
				Move to...
			</ContextMenu.Item>
			<ContextMenu.Item onclick={oncopyto}>
				<FolderOutputIcon />
				Copy to...
			</ContextMenu.Item>
			{#if sharesEnabled.enabled}
				<ContextMenu.Separator />
				<ContextMenu.Item onclick={onshare}>
					<Share2Icon />
					Share
				</ContextMenu.Item>
			{/if}
			{#if !item.isDir}
				<ContextMenu.Separator />
				<ContextMenu.Item onclick={handleDownload}>
					<DownloadIcon />
					Download
				</ContextMenu.Item>
				{#if versioningEnabled.enabled}
					<ContextMenu.Item onclick={onversions}>
						<HistoryIcon />
						Version history
					</ContextMenu.Item>
				{/if}
			{/if}
			<ContextMenu.Separator />
			<ContextMenu.Item variant="destructive" onclick={ondelete}>
				<Trash2Icon />
				Delete
				<ContextMenu.Shortcut>Del</ContextMenu.Shortcut>
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>
