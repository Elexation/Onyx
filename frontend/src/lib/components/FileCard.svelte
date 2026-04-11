<script lang="ts">
	import { goto } from "$app/navigation";
	import type { FileInfo } from "$lib/types";
	import { getDownloadUrl } from "$lib/api/files.js";
	import { formatFileSize } from "$lib/utils/format.js";
	import FileIcon from "./FileIcon.svelte";

	let { item }: { item: FileInfo } = $props();

	function handleClick() {
		if (item.isDir) {
			goto(`/files/${item.path}`);
		} else {
			const a = document.createElement("a");
			a.href = getDownloadUrl(item.path);
			a.download = item.name;
			a.click();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "Enter" || e.key === " ") {
			e.preventDefault();
			handleClick();
		}
	}
</script>

<div
	class="flex cursor-pointer flex-col items-center gap-2 rounded-lg border border-border/50 p-3 transition-colors hover:bg-accent/50"
	onclick={handleClick}
	onkeydown={handleKeydown}
	tabindex="0"
	role="button"
>
	<FileIcon mimeType={item.mimeType} isDir={item.isDir} class="size-10 text-muted-foreground" />
	<span class="w-full truncate text-center text-xs">{item.name}</span>
	{#if !item.isDir}
		<span class="text-[10px] text-muted-foreground">{formatFileSize(item.size)}</span>
	{/if}
</div>
