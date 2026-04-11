<script lang="ts">
	import { createVirtualizer } from "@tanstack/svelte-virtual";
	import type { Snippet } from "svelte";

	let {
		items,
		estimateSize = () => 40,
		overscan = 5,
		row,
	}: {
		items: unknown[];
		estimateSize?: () => number;
		overscan?: number;
		row: Snippet<[{ item: unknown; index: number; style: string }]>;
	} = $props();

	let scrollEl = $state<HTMLDivElement | null>(null);
	const makeGetScrollElement = (el: HTMLDivElement | null) => () => el;

	let virtualizer = $derived(
		createVirtualizer({
			count: items.length,
			getScrollElement: makeGetScrollElement(scrollEl),
			estimateSize,
			overscan,
		}),
	);
</script>

<div bind:this={scrollEl} class="min-h-0 flex-1 overflow-auto">
	<div class="relative w-full" style="height: {$virtualizer.getTotalSize()}px;">
		{#each $virtualizer.getVirtualItems() as vItem (vItem.index)}
			{@render row({
				item: items[vItem.index],
				index: vItem.index,
				style: `position: absolute; top: 0; left: 0; width: 100%; height: ${vItem.size}px; transform: translateY(${vItem.start}px);`,
			})}
		{/each}
	</div>
</div>
