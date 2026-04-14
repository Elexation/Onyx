<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import FolderPlusIcon from "@lucide/svelte/icons/folder-plus";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import ScissorsIcon from "@lucide/svelte/icons/scissors";
	import ClipboardPasteIcon from "@lucide/svelte/icons/clipboard-paste";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import type { Snippet } from "svelte";

	let {
		onnewfolder,
		ondelete,
		onpaste,
		oncopy,
		oncut,
		viewControls,
	}: {
		onnewfolder: () => void;
		ondelete: () => void;
		onpaste: () => void;
		oncopy: () => void;
		oncut: () => void;
		viewControls: Snippet;
	} = $props();
</script>

<div class="flex items-center gap-2">
	<Button variant="outline" size="sm" onclick={onnewfolder}>
		<FolderPlusIcon class="size-4" />
		New Folder
	</Button>

	{#if selection.isActive}
		<div class="flex items-center gap-1 rounded-md border border-border px-2 py-1">
			<span class="text-xs text-muted-foreground">{selection.count} selected</span>
			<Button variant="ghost" size="icon-xs" onclick={oncopy} title="Copy">
				<CopyIcon class="size-3.5" />
			</Button>
			<Button variant="ghost" size="icon-xs" onclick={oncut} title="Cut">
				<ScissorsIcon class="size-3.5" />
			</Button>
			{#if clipboard.hasItems}
				<Button variant="ghost" size="icon-xs" onclick={onpaste} title="Paste">
					<ClipboardPasteIcon class="size-3.5" />
				</Button>
			{/if}
			<Button variant="ghost" size="icon-xs" onclick={ondelete} title="Delete">
				<Trash2Icon class="size-3.5 text-destructive" />
			</Button>
		</div>
	{:else if clipboard.hasItems}
		<Button variant="outline" size="sm" onclick={onpaste}>
			<ClipboardPasteIcon class="size-4" />
			Paste
		</Button>
	{/if}

	<div class="ml-auto">
		{@render viewControls()}
	</div>
</div>
