<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import VirtualGrid from "./VirtualGrid.svelte";
	import FileCard from "./FileCard.svelte";
	import { setupMarquee } from "$lib/actions/marquee.js";

	let {
		items,
		onopen,
		onrename,
		ondelete,
		onpaste,
		onmoveto,
		oncopyto,
		onversions,
		onshare,
		ondrop,
	}: {
		items: FileInfo[];
		onopen: (item: FileInfo) => void;
		onrename: (item: FileInfo) => void;
		ondelete: (paths: string[]) => void;
		onpaste: () => void;
		onmoveto: (paths: string[]) => void;
		oncopyto: (paths: string[]) => void;
		onversions: (item: FileInfo) => void;
		onshare: (item: FileInfo) => void;
		ondrop: (paths: string[], destination: string) => void;
	} = $props();

	const allPaths = $derived(items.filter((i) => i.name !== "..").map((i) => i.path));

	let scrollEl = $state<HTMLDivElement | null>(null);
	const itemWidth = 160;
	const itemHeight = 140;
	const gap = 8;

	$effect(() => {
		if (!scrollEl) return;
		return setupMarquee(scrollEl, {
			getLayout: () => {
				const containerWidth = scrollEl!.clientWidth;
				const columns = Math.max(1, Math.floor((containerWidth + gap) / (itemWidth + gap)));
				return { mode: "grid", itemWidth, itemHeight, gap, paddingX: gap, columns };
			},
			getItems: () => items,
		});
	});
</script>

{#if items.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<VirtualGrid {items} bind:scrollEl>
		{#snippet cell({ item })}
			<FileCard
				item={item as FileInfo}
				{allPaths}
				{onopen}
				{onrename}
				{ondelete}
				{onpaste}
				{onmoveto}
				{oncopyto}
				{onversions}
				{onshare}
				{ondrop}
			/>
		{/snippet}
	</VirtualGrid>
{/if}
