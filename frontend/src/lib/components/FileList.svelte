<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { ArrowUp, ArrowDown } from "lucide-svelte";
	import type { SortField } from "$lib/stores/preferences.svelte.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import FileIcon from "./FileIcon.svelte";
	import FileContextMenu from "./FileContextMenu.svelte";
	import VirtualList from "./VirtualList.svelte";
	import EllipsisVerticalIcon from "@lucide/svelte/icons/ellipsis-vertical";
	import { longpress } from "$lib/actions/longpress.js";
	import { draggable } from "$lib/actions/draggable.js";
	import { droppable } from "$lib/actions/droppable.js";

	let {
		items,
		onopen,
		onrename,
		ondelete,
		onpaste,
		onmoveto,
		oncopyto,
		ondrop,
	}: {
		items: FileInfo[];
		onopen: (item: FileInfo) => void;
		onrename: (item: FileInfo) => void;
		ondelete: (paths: string[]) => void;
		onpaste: () => void;
		onmoveto: (paths: string[]) => void;
		oncopyto: (paths: string[]) => void;
		ondrop: (paths: string[], destination: string) => void;
	} = $props();

	const allPaths = $derived(items.filter((i) => i.name !== "..").map((i) => i.path));
	const allSelected = $derived(allPaths.length > 0 && selection.count === allPaths.length && allPaths.every((p) => selection.has(p)));
	const someSelected = $derived(selection.count > 0 && !allSelected);

	function toggleSelectAll() {
		if (allSelected) {
			selection.clear();
		} else {
			selection.selectAll(allPaths);
		}
	}

	const columns: { label: string; field: SortField; align: string; width: string }[] = [
		{ label: "Name", field: "name", align: "text-left", width: "" },
		{ label: "Size", field: "size", align: "text-right", width: "w-24" },
		{ label: "Modified", field: "modified", align: "text-right", width: "w-44" },
	];

	function handleSort(field: SortField) {
		if (preferences.sortField === field) {
			preferences.sortDir = preferences.sortDir === "asc" ? "desc" : "asc";
		} else {
			preferences.sortField = field;
			preferences.sortDir = "asc";
		}
	}

	function handleRowClick(e: MouseEvent, item: FileInfo) {
		e.stopPropagation();
		if (e.shiftKey) {
			e.preventDefault();
			selection.selectRange(item.path, allPaths);
		} else if (e.ctrlKey || e.metaKey) {
			e.preventDefault();
			selection.toggle(item.path);
		} else {
			selection.select(item.path);
		}
	}

	function handleRowKeydown(e: KeyboardEvent, item: FileInfo) {
		if (e.key === "Enter") {
			e.preventDefault();
			onopen(item);
		} else if (e.key === " ") {
			e.preventDefault();
			selection.toggle(item.path);
		}
	}

	function getContextPaths(item: FileInfo): string[] {
		return selection.has(item.path) && selection.count > 1
			? [...selection.items]
			: [item.path];
	}
</script>

{#if items.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
	<div class="flex border-b border-border text-xs text-muted-foreground" onclick={(e) => e.stopPropagation()}>
		<div class="flex w-10 items-center py-2 pl-4">
			<Checkbox
				checked={allSelected ? true : someSelected ? "indeterminate" : false}
				onCheckedChange={toggleSelectAll}
			/>
		</div>
		{#each columns as col}
			<button
				class="flex items-center gap-1 py-2 font-medium transition-colors hover:text-foreground {col.align} {col.width} {col.width ? 'pr-4' : 'flex-1'}"
				onclick={() => handleSort(col.field)}
			>
				{#if col.align === "text-right"}<span class="flex-1"></span>{/if}
				{col.label}
				{#if preferences.sortField === col.field}
					{#if preferences.sortDir === "asc"}
						<ArrowUp class="size-3" />
					{:else}
						<ArrowDown class="size-3" />
					{/if}
				{/if}
			</button>
		{/each}
		<div class="kebab-spacer hidden w-8"></div>
	</div>

	<VirtualList {items} estimateSize={() => 41}>
		{#snippet row({ item, style })}
			{@const file = item as FileInfo}
			{#if file.name === ".."}
				<div
					class="flex cursor-pointer items-center border-b border-border/50 text-muted-foreground transition-colors select-none hover:bg-accent/50"
					{style}
					onclick={(e) => { e.stopPropagation(); onopen(file); }}
					onkeydown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onopen(file); } }}
					use:droppable={{ path: file.path, ondrop }}
					tabindex={0}
					role="row"
				>
					<div class="w-10 py-2 pl-4"></div>
					<div class="flex flex-1 items-center gap-2 py-2 text-sm">
						<FileIcon isDir={true} class="size-4 opacity-50" />
						..
					</div>
				</div>
			{:else}
				{@const isSelected = selection.has(file.path)}
				{@const isCut = clipboard.isCut(file.path)}
				<FileContextMenu
					item={file}
					onopen={() => onopen(file)}
					onrename={() => onrename(file)}
					ondelete={() => ondelete(getContextPaths(file))}
					{onpaste}
					onmoveto={() => onmoveto(getContextPaths(file))}
					oncopyto={() => oncopyto(getContextPaths(file))}
				>
					{#snippet children(triggerProps)}
						<div
							{...triggerProps}
							class="flex cursor-pointer items-center border-b border-border/50 transition-colors select-none
								{isSelected ? 'bg-accent/70' : 'hover:bg-accent/50'}
								{isCut ? 'opacity-50' : ''}"
							{style}
							onclick={(e) => handleRowClick(e, file)}
							ondblclick={(e) => { e.stopPropagation(); onopen(file); }}
							onkeydown={(e) => handleRowKeydown(e, file)}
							use:longpress={() => selection.toggle(file.path)}
							use:draggable={{ path: file.path, isDir: file.isDir }}
							use:droppable={{ path: file.path, ondrop, enabled: file.isDir }}
							tabindex={0}
							role="row"
						>
							<div class="flex w-10 items-center py-2 pl-4" onclick={(e) => e.stopPropagation()}>
								<Checkbox
									checked={isSelected}
									onCheckedChange={() => selection.toggle(file.path)}
								/>
							</div>
							<div class="flex flex-1 items-center gap-2 py-2 text-sm">
								<FileIcon mimeType={file.mimeType} isDir={file.isDir} />
								{file.name}
							</div>
							<div class="w-24 py-2 pr-4 text-right text-sm text-muted-foreground">
								{file.isDir ? "\u2014" : formatFileSize(file.size)}
							</div>
							<div class="w-44 py-2 pr-4 text-right text-sm text-muted-foreground">
								{formatDate(file.modTime)}
							</div>
							<div class="kebab-button hidden w-8 items-center justify-center">
								<button
									class="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
									onclick={(e) => {
										e.stopPropagation();
										const row = e.currentTarget.closest('[role="row"]');
										if (row) row.dispatchEvent(new PointerEvent('contextmenu', { bubbles: true, clientX: e.clientX, clientY: e.clientY }));
									}}
									tabindex={-1}
								>
									<EllipsisVerticalIcon class="size-4" />
								</button>
							</div>
						</div>
					{/snippet}
				</FileContextMenu>
			{/if}
		{/snippet}
	</VirtualList>
{/if}

<style>
	@media (pointer: coarse) and (hover: none) {
		.kebab-button { display: flex !important; }
		.kebab-spacer { display: block !important; }
	}
	:global(.drop-target-active) {
		background-color: hsl(var(--accent) / 0.5) !important;
		outline: 2px dashed hsl(var(--primary));
		outline-offset: -2px;
	}
</style>
