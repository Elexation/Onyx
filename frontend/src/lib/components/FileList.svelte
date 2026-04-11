<script lang="ts">
	import { goto } from "$app/navigation";
	import type { FileInfo } from "$lib/types";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import { ArrowUp, ArrowDown } from "lucide-svelte";
	import type { SortField } from "$lib/stores/preferences.svelte.js";
	import FileIcon from "./FileIcon.svelte";
	import VirtualList from "./VirtualList.svelte";

	let { items }: { items: FileInfo[] } = $props();

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

	function handleClick(item: FileInfo) {
		if (item.isDir) {
			goto(`/files/${item.path}`);
		} else {
			const a = document.createElement("a");
			a.href = getDownloadUrl(item.path);
			a.download = item.name;
			a.click();
		}
	}

	function handleKeydown(e: KeyboardEvent, item: FileInfo) {
		if (e.key === "Enter" || e.key === " ") {
			e.preventDefault();
			handleClick(item);
		}
	}
</script>

{#if items.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<div class="flex border-b border-border text-xs text-muted-foreground">
		<div class="w-10 py-2 pl-4"></div>
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
	</div>

	<VirtualList {items} estimateSize={() => 41}>
		{#snippet row({ item, style })}
			{@const file = item as FileInfo}
			<div
				class="flex cursor-pointer items-center border-b border-border/50 transition-colors hover:bg-accent/50"
				{style}
				onclick={() => handleClick(file)}
				onkeydown={(e) => handleKeydown(e, file)}
				tabindex="0"
				role="button"
			>
				<div class="w-10 py-2 pl-4">
					<FileIcon mimeType={file.mimeType} isDir={file.isDir} />
				</div>
				<div class="flex-1 py-2 text-sm">{file.name}</div>
				<div class="w-24 py-2 pr-4 text-right text-sm text-muted-foreground">
					{file.isDir ? "\u2014" : formatFileSize(file.size)}
				</div>
				<div class="w-44 py-2 pr-4 text-right text-sm text-muted-foreground">
					{formatDate(file.modTime)}
				</div>
			</div>
		{/snippet}
	</VirtualList>
{/if}
