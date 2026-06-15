<script lang="ts">
	import { createVirtualizer } from "@tanstack/svelte-virtual";
	import type { Snippet } from "svelte";

	let {
		items,
		itemWidth = 148,
		itemHeight = 210,
		gap = 10,
		overscan = 5,
		cell,
		scrollEl = $bindable<HTMLDivElement | null>(null),
	}: {
		items: unknown[];
		itemWidth?: number;
		itemHeight?: number;
		gap?: number;
		overscan?: number;
		cell: Snippet<[{ item: unknown; index: number }]>;
		scrollEl?: HTMLDivElement | null;
	} = $props();

	let containerWidth = $state(0);
	const makeGetScrollElement = (el: HTMLDivElement | null) => () => el;

	const columns = $derived(Math.max(1, Math.floor((containerWidth + gap) / (itemWidth + gap))));
	const rowCount = $derived(Math.ceil(items.length / columns));

	// keep `count: rowCount` alone — do NOT pass `lanes: columns` (that's
	// masonry; items stack silently at y=0 when items < lanes).
	let virtualizer = $derived(
		createVirtualizer({
			count: rowCount,
			getScrollElement: makeGetScrollElement(scrollEl),
			estimateSize: () => itemHeight + gap,
			overscan,
		}),
	);
</script>

<div bind:this={scrollEl} bind:clientWidth={containerWidth} class="min-h-0 flex-1 overflow-auto">
	<div class="relative w-full" style="height: {$virtualizer.getTotalSize()}px;">
		{#each $virtualizer.getVirtualItems() as vRow (vRow.index)}
			{@const rowStart = vRow.index * columns}
			<div
				class="absolute top-0 left-0 grid w-full"
				style="transform: translateY({vRow.start}px); gap: {gap}px; grid-template-columns: repeat({columns}, minmax(0, 1fr));"
			>
				{#each Array(columns) as _, col}
					{@const itemIndex = rowStart + col}
					{#if itemIndex < items.length}
						<div style="height: {itemHeight}px;">
							{@render cell({ item: items[itemIndex], index: itemIndex })}
						</div>
					{/if}
				{/each}
			</div>
		{/each}
	</div>
</div>
