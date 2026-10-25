<script lang="ts">
	import { createVirtualizer } from "@tanstack/svelte-virtual";
	import { get } from "svelte/store";
	import type { Snippet } from "svelte";

	let {
		items,
		estimateSize = () => 40,
		overscan = 5,
		row,
		scrollEl = $bindable<HTMLDivElement | null>(null),
		externalScrollEl = null,
		sizeKey = undefined,
	}: {
		items: unknown[];
		estimateSize?: () => number;
		overscan?: number;
		row: Snippet<[{ item: unknown; index: number; style: string }]>;
		scrollEl?: HTMLDivElement | null;
		externalScrollEl?: HTMLElement | null;
		// Change it when estimateSize's result changes (e.g. mobile row height)
		sizeKey?: unknown;
	} = $props();

	// Create once, then setOptions: re-creating drops the measurement cache every refresh
	const virtualizer = createVirtualizer({
		count: 0,
		getScrollElement: () => externalScrollEl ?? scrollEl,
		estimateSize: () => 40,
		overscan: 5,
	});

	let measuredKey: unknown = undefined;
	$effect(() => {
		const key = sizeKey;
		const v = get(virtualizer);
		v.setOptions({
			count: items.length,
			getScrollElement: () => externalScrollEl ?? scrollEl,
			estimateSize,
			overscan,
		});
		// Cached row sizes ignore estimateSize changes; drop them when the key flips
		if (key !== measuredKey) {
			measuredKey = key;
			v.measure();
		}
	});
</script>

{#if externalScrollEl}
	<div class="relative w-full" style="height: {$virtualizer.getTotalSize()}px;">
		{#each $virtualizer.getVirtualItems() as vItem (vItem.index)}
			{#if vItem.index < items.length}
				{@render row({
					item: items[vItem.index],
					index: vItem.index,
					style: `position: absolute; top: 0; left: 0; width: 100%; height: ${vItem.size}px; transform: translateY(${vItem.start}px);`,
				})}
			{/if}
		{/each}
	</div>
{:else}
	<div bind:this={scrollEl} class="min-h-0 flex-1 overflow-auto">
		<div class="relative w-full" style="height: {$virtualizer.getTotalSize()}px;">
			{#each $virtualizer.getVirtualItems() as vItem (vItem.index)}
				{#if vItem.index < items.length}
					{@render row({
						item: items[vItem.index],
						index: vItem.index,
						style: `position: absolute; top: 0; left: 0; width: 100%; height: ${vItem.size}px; transform: translateY(${vItem.start}px);`,
					})}
				{/if}
			{/each}
		</div>
	</div>
{/if}
