<script lang="ts">
	import * as ContextMenu from "$lib/components/ui/context-menu/index.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { getDownloadUrl } from "$lib/api/files.js";
	import type { Snippet } from "svelte";

	let {
		item,
		onopen,
		onrename,
		ondelete,
		onmoveto,
		oncopyto,
		onpaste,
		children,
	}: {
		item: { name: string; path: string; isDir: boolean } | null;
		onopen: () => void;
		onrename: () => void;
		ondelete: () => void;
		onmoveto: () => void;
		oncopyto: () => void;
		onpaste: () => void;
		children: Snippet<[Record<string, any>]>;
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
	<ContextMenu.Trigger>
		{#snippet child({ props })}
			{@render children(props)}
		{/snippet}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52">
		{#if item}
			<ContextMenu.Item onclick={onopen}>
				Open
				<ContextMenu.Shortcut>Enter</ContextMenu.Shortcut>
			</ContextMenu.Item>
			<ContextMenu.Separator />
			{#if selection.count <= 1}
				<ContextMenu.Item onclick={onrename}>
					Rename
					<ContextMenu.Shortcut>F2</ContextMenu.Shortcut>
				</ContextMenu.Item>
			{/if}
			<ContextMenu.Item onclick={handleCopy}>
				Copy
				<ContextMenu.Shortcut>Ctrl+C</ContextMenu.Shortcut>
			</ContextMenu.Item>
			<ContextMenu.Item onclick={handleCut}>
				Cut
				<ContextMenu.Shortcut>Ctrl+X</ContextMenu.Shortcut>
			</ContextMenu.Item>
			{#if clipboard.hasItems}
				<ContextMenu.Separator />
			{/if}
		{/if}
		{#if clipboard.hasItems}
			<ContextMenu.Item onclick={onpaste}>
				Paste
				<ContextMenu.Shortcut>Ctrl+V</ContextMenu.Shortcut>
			</ContextMenu.Item>
		{/if}
		{#if item}
			<ContextMenu.Separator />
			<ContextMenu.Item onclick={onmoveto}>Move to...</ContextMenu.Item>
			<ContextMenu.Item onclick={oncopyto}>Copy to...</ContextMenu.Item>
			{#if !item.isDir}
				<ContextMenu.Separator />
				<ContextMenu.Item onclick={handleDownload}>Download</ContextMenu.Item>
			{/if}
			<ContextMenu.Separator />
			<ContextMenu.Item variant="destructive" onclick={ondelete}>
				Delete
				<ContextMenu.Shortcut>Del</ContextMenu.Shortcut>
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>
