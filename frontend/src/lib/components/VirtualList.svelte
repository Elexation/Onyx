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
	}: {
		items: unknown[];
		estimateSize?: () => number;
		overscan?: number;
		row: Snippet<[{ item: unknown; index: number; style: string }]>;
		scrollEl?: HTMLDivElement | null;
		externalScrollEl?: HTMLElement | null;
	} = $props();

	// Create the virtualizer once — push prop updates via setOptions in $effect
	// instead of re-creating it inside $derived. Re-creation discards the
	// row-measurement cache and resize/intersection observers on every items
	// reference change (which sorted=$derived produces on every listing refresh
	// / change-feed tick). Placeholder count=0 avoids reading reactive props at
	// script body; $effect populates real values synchronously on mount.
	const virtualizer = createVirtualizer({
		count: 0,
		getScrollElement: () => externalScrollEl ?? scrollEl,
		estimateSize: () => 40,
		overscan: 5,
	});

	$effect(() => {
		get(virtualizer).setOptions({
			count: items.length,
			getScrollElement: () => externalScrollEl ?? scrollEl,
			estimateSize,
			overscan,
		});
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
