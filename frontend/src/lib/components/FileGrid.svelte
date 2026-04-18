<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import VirtualGrid from "./VirtualGrid.svelte";
	import FileCard from "./FileCard.svelte";

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
</script>

{#if items.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<VirtualGrid {items}>
		{#snippet cell({ item })}
			<FileCard
				item={item as FileInfo}
				{onopen}
				{onrename}
				{ondelete}
				{onpaste}
				{onmoveto}
				{oncopyto}
				{ondrop}
			/>
		{/snippet}
	</VirtualGrid>
{/if}
