<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { formatFileSize } from "$lib/utils/format.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import FileIcon from "./FileIcon.svelte";
	import FileContextMenu from "./FileContextMenu.svelte";
	import EllipsisVerticalIcon from "@lucide/svelte/icons/ellipsis-vertical";
	import { longpress } from "$lib/actions/longpress.js";

	let {
		item,
		onopen,
		onrename,
		ondelete,
		onpaste,
		onmoveto,
		oncopyto,
	}: {
		item: FileInfo;
		onopen: (item: FileInfo) => void;
		onrename: (item: FileInfo) => void;
		ondelete: (paths: string[]) => void;
		onpaste: () => void;
		onmoveto: (paths: string[]) => void;
		oncopyto: (paths: string[]) => void;
	} = $props();

	const isSelected = $derived(selection.has(item.path));
	const isCut = $derived(clipboard.isCut(item.path));

	function handleClick(e: MouseEvent) {
		if (e.shiftKey) {
			e.preventDefault();
			// Range selection needs all paths — done at parent level
		} else if (e.ctrlKey || e.metaKey) {
			e.preventDefault();
			selection.toggle(item.path);
		} else if (selection.isActive) {
			selection.select(item.path);
		} else {
			onopen(item);
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "Enter" || e.key === " ") {
			e.preventDefault();
			if (selection.isActive) {
				selection.select(item.path);
			} else {
				onopen(item);
			}
		}
	}

	function getContextPaths(): string[] {
		return selection.has(item.path) && selection.count > 1
			? [...selection.items]
			: [item.path];
	}
</script>

{#if item.name === ".."}
	<div
		class="flex cursor-pointer flex-col items-center gap-2 rounded-lg border border-border/50 p-3 text-muted-foreground transition-colors select-none hover:bg-accent/50"
		onclick={() => onopen(item)}
		onkeydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onopen(item); } }}
		tabindex={0}
		role="gridcell"
	>
		<FileIcon isDir={true} class="size-10 text-muted-foreground opacity-50" />
		<span class="w-full truncate text-center text-xs">..</span>
	</div>
{:else}
	<FileContextMenu
		{item}
		onopen={() => onopen(item)}
		onrename={() => onrename(item)}
		ondelete={() => ondelete(getContextPaths())}
		{onpaste}
		onmoveto={() => onmoveto(getContextPaths())}
		oncopyto={() => oncopyto(getContextPaths())}
	>
		{#snippet children(triggerProps)}
			<div
				{...triggerProps}
				class="relative flex cursor-pointer flex-col items-center gap-2 rounded-lg border border-border/50 p-3 transition-colors select-none
					{isSelected ? 'bg-accent/70 border-accent' : 'hover:bg-accent/50'}
					{isCut ? 'opacity-50' : ''}"
				onclick={handleClick}
				onkeydown={handleKeydown}
				use:longpress={() => selection.toggle(item.path)}
				tabindex={0}
				role="gridcell"
			>
				{#if selection.isActive}
					<div
						class="absolute left-2 top-2"
						role="presentation"
						onclick={(e) => { e.stopPropagation(); selection.toggle(item.path); }}
					>
						<Checkbox checked={isSelected} />
					</div>
				{/if}
				<div class="kebab-button absolute right-1 top-1 hidden">
					<button
						class="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
						onclick={(e) => {
							e.stopPropagation();
							const card = e.currentTarget.closest('[role="gridcell"]');
							if (card) card.dispatchEvent(new PointerEvent('contextmenu', { bubbles: true, clientX: e.clientX, clientY: e.clientY }));
						}}
						tabindex={-1}
					>
						<EllipsisVerticalIcon class="size-4" />
					</button>
				</div>
				<FileIcon mimeType={item.mimeType} isDir={item.isDir} class="size-10 text-muted-foreground" />
				<span class="w-full truncate text-center text-xs">{item.name}</span>
				{#if !item.isDir}
					<span class="text-[10px] text-muted-foreground">{formatFileSize(item.size)}</span>
				{/if}
			</div>
		{/snippet}
	</FileContextMenu>
{/if}

<style>
	@media (pointer: coarse) and (hover: none) {
		.kebab-button { display: block !important; }
	}
</style>
