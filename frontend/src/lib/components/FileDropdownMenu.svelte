<script lang="ts">
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { versioningEnabled } from "$lib/stores/versioningEnabled.svelte.js";
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
		document.body.appendChild(a);
		a.click();
		a.remove();
	}
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			{@render trigger(props)}
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content class="w-52" align="end">
		{#if item}
			<DropdownMenu.Item onclick={onopen}>
				Open
				<DropdownMenu.Shortcut>Enter</DropdownMenu.Shortcut>
			</DropdownMenu.Item>
			<DropdownMenu.Separator />
			{#if selection.count <= 1}
				<DropdownMenu.Item onclick={onrename}>
					Rename
					<DropdownMenu.Shortcut>F2</DropdownMenu.Shortcut>
				</DropdownMenu.Item>
			{/if}
			<DropdownMenu.Item onclick={handleCopy}>
				Copy
				<DropdownMenu.Shortcut>Ctrl+C</DropdownMenu.Shortcut>
			</DropdownMenu.Item>
			<DropdownMenu.Item onclick={handleCut}>
				Cut
				<DropdownMenu.Shortcut>Ctrl+X</DropdownMenu.Shortcut>
			</DropdownMenu.Item>
			{#if clipboard.hasItems}
				<DropdownMenu.Separator />
			{/if}
		{/if}
		{#if clipboard.hasItems}
			<DropdownMenu.Item onclick={onpaste}>
				Paste
				<DropdownMenu.Shortcut>Ctrl+V</DropdownMenu.Shortcut>
			</DropdownMenu.Item>
		{/if}
		{#if item}
			<DropdownMenu.Separator />
			<DropdownMenu.Item onclick={onmoveto}>Move to...</DropdownMenu.Item>
			<DropdownMenu.Item onclick={oncopyto}>Copy to...</DropdownMenu.Item>
			{#if sharesEnabled.enabled}
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={onshare}>Share</DropdownMenu.Item>
			{/if}
			{#if !item.isDir}
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={handleDownload}>Download</DropdownMenu.Item>
				{#if versioningEnabled.enabled}
					<DropdownMenu.Item onclick={onversions}>Version history</DropdownMenu.Item>
				{/if}
			{/if}
			<DropdownMenu.Separator />
			<DropdownMenu.Item variant="destructive" onclick={ondelete}>
				Delete
				<DropdownMenu.Shortcut>Del</DropdownMenu.Shortcut>
			</DropdownMenu.Item>
		{/if}
	</DropdownMenu.Content>
</DropdownMenu.Root>
