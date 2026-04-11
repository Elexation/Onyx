<script lang="ts">
	import { createVirtualizer } from "@tanstack/svelte-virtual";
	import type { Snippet } from "svelte";

	let {
		items,
		itemWidth = 160,
		itemHeight = 140,
		gap = 8,
		overscan = 5,
		cell,
	}: {
		items: unknown[];
		itemWidth?: number;
		itemHeight?: number;
		gap?: number;
		overscan?: number;
		cell: Snippet<[{ item: unknown; index: number }]>;
	} = $props();

	let scrollEl = $state<HTMLDivElement | null>(null);
	let containerWidth = $state(0);
	const makeGetScrollElement = (el: HTMLDivElement | null) => () => el;

	const columns = $derived(Math.max(1, Math.floor((containerWidth + gap) / (itemWidth + gap))));
	const rowCount = $derived(Math.ceil(items.length / columns));

	let virtualizer = $derived(
		createVirtualizer({
			count: rowCount,
			getScrollElement: makeGetScrollElement(scrollEl),
			estimateSize: () => itemHeight + gap,
			overscan,
			lanes: columns,
		}),
	);
</script>

<div bind:this={scrollEl} bind:clientWidth={containerWidth} class="min-h-0 flex-1 overflow-auto">
	<div class="relative w-full" style="height: {$virtualizer.getTotalSize()}px;">
		{#each $virtualizer.getVirtualItems() as vRow (vRow.index)}
			{@const rowStart = vRow.index * columns}
			<div
				class="absolute left-0 top-0 flex w-full"
				style="height: {itemHeight}px; transform: translateY({vRow.start}px); gap: {gap}px; padding: 0 {gap}px;"
			>
				{#each Array(columns) as _, col}
					{@const itemIndex = rowStart + col}
					{#if itemIndex < items.length}
						<div style="width: {itemWidth}px; flex-shrink: 0;">
							{@render cell({ item: items[itemIndex], index: itemIndex })}
						</div>
					{/if}
				{/each}
			</div>
		{/each}
	</div>
</div>
