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
	<button
		type="button"
		class="inline-flex size-[30px] items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
		onclick={() => (preferences.showHidden = !preferences.showHidden)}
		title={preferences.showHidden ? "Hide hidden files" : "Show hidden files"}
		aria-pressed={preferences.showHidden}
	>
		{#if preferences.showHidden}
			<Eye class="size-[15px]" strokeWidth={2} />
		{:else}
			<EyeOff class="size-[15px]" strokeWidth={2} />
		{/if}
	</button>

	<div class="inline-flex rounded-lg border border-border-2 bg-card p-[2px]">
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors {viewMode ===
			'grid'
				? 'bg-muted text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
			onclick={() => onviewchange("grid")}
			title="Grid view"
			aria-pressed={viewMode === "grid"}
		>
			<LayoutGrid class="size-[15px]" strokeWidth={2} />
		</button>
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors {viewMode ===
			'list'
				? 'bg-muted text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
			onclick={() => onviewchange("list")}
			title="List view"
			aria-pressed={viewMode === "list"}
		>
			<List class="size-[15px]" strokeWidth={2} />
		</button>
	</div>
</div>
