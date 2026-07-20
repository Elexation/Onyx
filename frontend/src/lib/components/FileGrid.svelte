<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import VirtualGrid from "./VirtualGrid.svelte";
	import FileCard from "./FileCard.svelte";
	import EmptyState from "./EmptyState.svelte";
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
	const minItemWidth = 148;
	const itemHeight = 168;
	const gap = 10;

	$effect(() => {
		if (!scrollEl) return;
		return setupMarquee(scrollEl, {
			getLayout: () => {
				const containerWidth = scrollEl!.clientWidth;
				const columns = Math.max(
					1,
					Math.floor((containerWidth + gap) / (minItemWidth + gap)),
				);
				// Grid uses 1fr columns, so actual rendered column width ≠ minItemWidth.
				// Marquee hit-test needs the real stride.
				const actualItemWidth = (containerWidth - (columns - 1) * gap) / columns;
				return {
					mode: "grid",
					itemWidth: actualItemWidth,
					itemHeight,
					gap,
					paddingX: 0,
					columns,
				};
			},
			getItems: () => items,
		});
	});
</script>

{#if items.length === 0}
	<EmptyState title="This folder is empty" />
{:else}
	<VirtualGrid {items} itemWidth={minItemWidth} {itemHeight} {gap} bind:scrollEl>
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
