<script lang="ts">
	import { goto } from "$app/navigation";
	import type { FileInfo } from "$lib/types";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import FileIcon from "./FileIcon.svelte";

	let { items = [] }: { items: FileInfo[] } = $props();

	const sorted = $derived.by(() => {
		const dirs = items.filter((f) => f.isDir);
		const files = items.filter((f) => !f.isDir);
		dirs.sort((a, b) => a.name.localeCompare(b.name));
		files.sort((a, b) => a.name.localeCompare(b.name));
		return [...dirs, ...files];
	});

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

{#if sorted.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<table class="w-full">
		<thead>
			<tr class="border-b border-border text-left text-xs text-muted-foreground">
				<th class="w-10 py-2 pl-4"></th>
				<th class="py-2 font-medium">Name</th>
				<th class="w-24 py-2 pr-4 text-right font-medium">Size</th>
				<th class="w-44 py-2 pr-4 text-right font-medium">Modified</th>
			</tr>
		</thead>
		<tbody>
			{#each sorted as item (item.path)}
				<tr
					class="cursor-pointer border-b border-border/50 transition-colors hover:bg-accent/50"
					onclick={() => handleClick(item)}
					onkeydown={(e) => handleKeydown(e, item)}
					tabindex="0"
					role="button"
				>
					<td class="py-2 pl-4">
						<FileIcon mimeType={item.mimeType} isDir={item.isDir} />
					</td>
					<td class="py-2 text-sm">{item.name}</td>
					<td class="py-2 pr-4 text-right text-sm text-muted-foreground">
						{item.isDir ? "\u2014" : formatFileSize(item.size)}
					</td>
					<td class="py-2 pr-4 text-right text-sm text-muted-foreground">
						{formatDate(item.modTime)}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}
