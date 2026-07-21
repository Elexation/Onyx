<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import VirtualGrid from "./VirtualGrid.svelte";
	import FileCard from "./FileCard.svelte";
	import EmptyState from "./EmptyState.svelte";
	import { setupMarquee } from "$lib/actions/marquee.js";

	let {
		items,
		highlightName = null,
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
		highlightName?: string | null;
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

	$effect(() => {
		if (!highlightName || items.length === 0 || !scrollEl) return;
		const idx = items.findIndex((i) => i.name === highlightName);
		if (idx === -1) return;

		const el = scrollEl;
		const name = highlightName;
		let rafId1: number, rafId2: number;
		let t1: ReturnType<typeof setTimeout>, t2: ReturnType<typeof setTimeout>, t3: ReturnType<typeof setTimeout>;

		rafId1 = requestAnimationFrame(() => {
			const containerWidth = el.clientWidth;
			const cols = Math.max(1, Math.floor((containerWidth + gap) / (minItemWidth + gap)));
			const row = Math.floor(idx / cols);
			const targetTop = row * (itemHeight + gap);
			const centerOffset = el.clientHeight / 2 - itemHeight / 2;
			el.scrollTo({ top: Math.max(0, targetTop - centerOffset), behavior: "instant" });

			t1 = setTimeout(() => {
				const card = el.querySelector<HTMLElement>(`[data-file-name="${CSS.escape(name)}"]`);
				if (card) {
					card.style.transition = "none";
					card.style.backgroundColor = "oklch(0.74 0.13 245 / 0.3)";
					t2 = setTimeout(() => {
						card.style.transition = "background-color 1.5s ease-out";
						rafId2 = requestAnimationFrame(() => {
							card.style.backgroundColor = "";
						});
						t3 = setTimeout(() => {
							card.style.transition = "";
						}, 1600);
					}, 600);
				}
			}, 50);
		});

		return () => {
			cancelAnimationFrame(rafId1);
			cancelAnimationFrame(rafId2);
			clearTimeout(t1);
			clearTimeout(t2);
			clearTimeout(t3);
		};
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
