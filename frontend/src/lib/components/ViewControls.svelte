<script lang="ts">
	import { List, LayoutGrid, Eye, EyeOff } from "lucide-svelte";
	import { preferences, type ViewMode } from "$lib/stores/preferences.svelte.js";

	let {
		viewMode,
		onviewchange,
	}: {
		viewMode: ViewMode;
		onviewchange: (mode: ViewMode) => void;
	} = $props();

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
