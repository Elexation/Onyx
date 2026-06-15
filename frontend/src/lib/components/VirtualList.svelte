<script lang="ts">
	import { createVirtualizer } from "@tanstack/svelte-virtual";
	import type { Snippet } from "svelte";

	let {
		items,
		estimateSize = () => 40,
		overscan = 5,
		row,
		scrollEl = $bindable<HTMLDivElement | null>(null),
		externalScrollEl = null,
	}: {
		items: unknown[];
		estimateSize?: () => number;
		overscan?: number;
		row: Snippet<[{ item: unknown; index: number; style: string }]>;
		scrollEl?: HTMLDivElement | null;
		externalScrollEl?: HTMLElement | null;
	} = $props();

	const makeGetScrollElement = (el: HTMLElement | null) => () => el;

	let virtualizer = $derived(
		createVirtualizer({
			count: items.length,
			getScrollElement: makeGetScrollElement(externalScrollEl ?? scrollEl),
			estimateSize,
			overscan,
		}),
	);
</script>

{#if externalScrollEl}
	<div class="relative w-full" style="height: {$virtualizer.getTotalSize()}px;">
		{#each $virtualizer.getVirtualItems() as vItem (vItem.index)}
			{@render row({
				item: items[vItem.index],
				index: vItem.index,
				style: `position: absolute; top: 0; left: 0; width: 100%; height: ${vItem.size}px; transform: translateY(${vItem.start}px);`,
			})}
		{/each}
	</div>
{:else}
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
{/if}
