<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import VirtualGrid from "./VirtualGrid.svelte";
	import FileCard from "./FileCard.svelte";

	let { items }: { items: FileInfo[] } = $props();
</script>

{#if items.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<VirtualGrid {items}>
		{#snippet cell({ item })}
			<FileCard item={item as FileInfo} />
		{/snippet}
	</VirtualGrid>
{/if}
