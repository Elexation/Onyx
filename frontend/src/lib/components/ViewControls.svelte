<script lang="ts">
	import { List, LayoutGrid, Eye, EyeOff, ArrowUpDown } from "lucide-svelte";
	import { preferences, type SortField, type SortDir, type ViewMode } from "$lib/stores/preferences.svelte.js";

	let {
		viewMode,
		onviewchange,
	}: {
		viewMode: ViewMode;
		onviewchange: (mode: ViewMode) => void;
	} = $props();

	const sortOptions: { label: string; field: SortField }[] = [
		{ label: "Name", field: "name" },
		{ label: "Size", field: "size" },
		{ label: "Modified", field: "modified" },
		{ label: "Type", field: "type" },
	];

	function cycleSort(field: SortField) {
		if (preferences.sortField === field) {
			preferences.sortDir = preferences.sortDir === "asc" ? "desc" : "asc";
		} else {
			preferences.sortField = field;
			preferences.sortDir = "asc";
		}
	}
</script>

<div class="flex items-center gap-2">
	<div class="flex items-center gap-1 rounded-md border border-border p-0.5">
		<button
			class="rounded p-1 transition-colors {viewMode === 'list' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}"
			onclick={() => onviewchange("list")}
			title="List view"
		>
			<List class="size-4" />
		</button>
		<button
			class="rounded p-1 transition-colors {viewMode === 'grid' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}"
			onclick={() => onviewchange("grid")}
			title="Grid view"
		>
			<LayoutGrid class="size-4" />
		</button>
	</div>

	<div class="flex items-center gap-1">
		<ArrowUpDown class="size-3.5 text-muted-foreground" />
		{#each sortOptions as opt}
			<button
				class="rounded px-2 py-1 text-xs transition-colors {preferences.sortField === opt.field ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}"
				onclick={() => cycleSort(opt.field)}
				title="Sort by {opt.label}"
			>
				{opt.label}
				{#if preferences.sortField === opt.field}
					<span class="ml-0.5">{preferences.sortDir === "asc" ? "\u2191" : "\u2193"}</span>
				{/if}
			</button>
		{/each}
	</div>

	<button
		class="ml-auto rounded p-1 transition-colors {preferences.showHidden ? 'text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}"
		onclick={() => (preferences.showHidden = !preferences.showHidden)}
		title="{preferences.showHidden ? 'Hide' : 'Show'} hidden files"
	>
		{#if preferences.showHidden}
			<Eye class="size-4" />
		{:else}
			<EyeOff class="size-4" />
		{/if}
	</button>
</div>
