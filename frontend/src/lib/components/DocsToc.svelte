<script lang="ts">
	import { onMount, tick } from "svelte";
	import { docsDrawer } from "$lib/stores/docsDrawer.svelte.js";
	import { allSections, scopeColors } from "$lib/docs/api-sections.js";
	import MobileDrawer from "$lib/components/MobileDrawer.svelte";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import SearchIcon from "@lucide/svelte/icons/search";
	import XIcon from "@lucide/svelte/icons/x";

	let activeSection = $state("authentication");
	let expandedGroups = $state<Record<string, boolean>>({
		overview: false,
		read: false,
		upload: false,
		full: false,
	});

	let scrollEl: Element | null = null;

	function recalcActiveSection() {
		if (!scrollEl) return;
		const containerTop = scrollEl.getBoundingClientRect().top;
		const threshold = containerTop + 100;
		let active: string | null = null;
		let firstVisible: string | null = null;
		for (const s of allSections) {
			const el = document.getElementById(s.id);
			if (!el || !el.offsetHeight) continue;
			if (!firstVisible) firstVisible = s.id;
			if (el.getBoundingClientRect().top <= threshold) {
				active = s.id;
			}
		}
		activeSection = active ?? firstVisible ?? allSections[0].id;
	}

	onMount(() => {
		scrollEl = document.getElementById("docs-scroll");
		if (!scrollEl) return;
		scrollEl.addEventListener("scroll", recalcActiveSection, { passive: true });
		return () => scrollEl!.removeEventListener("scroll", recalcActiveSection);
	});

	$effect(() => {
		docsDrawer.filterQuery;
		tick().then(recalcActiveSection);
	});
</script>

{#snippet scopeBadge(scope: "read" | "upload" | "full")}
	<span
		class="shrink-0 rounded-[5px] px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] {scopeColors[scope]}"
	>
		{scope}
	</span>
{/snippet}

{#snippet tocNav()}
	<div class="relative mb-2">
		<SearchIcon class="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
		<input
			type="text"
			placeholder="Filter..."
			bind:value={docsDrawer.filterQuery}
			class="w-full rounded-lg border border-border bg-muted/30 py-1.5 pl-7 pr-7 text-meta text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-accent-brand"
		/>
		{#if docsDrawer.filterQuery}
			<button
				onclick={() => (docsDrawer.filterQuery = "")}
				class="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:text-foreground"
				aria-label="Clear filter"
			>
				<XIcon class="size-3.5" />
			</button>
		{/if}
	</div>
	<nav class="flex flex-col gap-0.5">
		{#if docsDrawer.filteredOverview.length > 0}
			<button
				onclick={() => (expandedGroups.overview = !expandedGroups.overview)}
				class="mb-0.5 flex w-full cursor-pointer items-center gap-1 px-2.5 text-[11px] font-semibold tracking-wider text-muted-foreground-2 uppercase hover:text-muted-foreground"
			>
				<ChevronDownIcon class="size-3 transition-transform {expandedGroups.overview || docsDrawer.filterQuery ? '' : '-rotate-90'}" />
				Overview
			</button>
			{#if expandedGroups.overview || docsDrawer.filterQuery}
				{#each docsDrawer.filteredOverview as s}
					<a
						href="#{s.id}"
						onclick={(e: MouseEvent) => {
							e.preventDefault();
							docsDrawer.open = false;
							document.getElementById(s.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
						}}
						class="rounded-lg px-2.5 py-1.5 pl-6 text-meta font-medium transition-colors {activeSection === s.id
							? 'bg-muted text-foreground'
							: 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
					>
						{s.label}
					</a>
				{/each}
			{/if}
		{/if}

		{#each docsDrawer.filteredGroups as group}
			<button
				onclick={() => (expandedGroups[group.scope] = !expandedGroups[group.scope])}
				class="mt-3 mb-0.5 flex w-full cursor-pointer items-center gap-1 px-2.5 text-[11px] font-semibold tracking-wider text-muted-foreground-2 uppercase hover:text-muted-foreground"
			>
				<ChevronDownIcon class="size-3 transition-transform {expandedGroups[group.scope] || docsDrawer.filterQuery ? '' : '-rotate-90'}" />
				<span class="flex items-center gap-1.5">
					{group.label}
					{@render scopeBadge(group.scope)}
				</span>
			</button>
			{#if expandedGroups[group.scope] || docsDrawer.filterQuery}
				{#each group.sections as s}
					<a
						href="#{s.id}"
						onclick={(e: MouseEvent) => {
							e.preventDefault();
							docsDrawer.open = false;
							document.getElementById(s.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
						}}
						class="rounded-lg px-2.5 py-1.5 pl-6 text-meta font-medium transition-colors {activeSection === s.id
							? 'bg-muted text-foreground'
							: 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
					>
						{s.label}
					</a>
				{/each}
			{/if}
		{/each}

		{#if docsDrawer.filterQuery && docsDrawer.filteredOverview.length === 0 && docsDrawer.filteredGroups.length === 0}
			<p class="px-2.5 py-3 text-meta text-muted-foreground">No matches</p>
		{/if}
	</nav>
{/snippet}

<MobileDrawer bind:open={docsDrawer.open}>
	<aside class="h-full w-[200px] shrink-0 overflow-y-auto border-r border-border bg-card p-3 lg:hidden">
		{@render tocNav()}
	</aside>
</MobileDrawer>

<aside class="hidden shrink-0 overflow-y-auto border-r border-border p-3 lg:block lg:w-[200px]">
	{@render tocNav()}
</aside>
