<script lang="ts">
	import ListIcon from "@lucide/svelte/icons/list";
	import LayoutGridIcon from "@lucide/svelte/icons/layout-grid";
	import type { ViewMode } from "$lib/stores/preferences.svelte.js";

	let {
		viewMode,
		onviewchange,
	}: {
		viewMode: ViewMode;
		onviewchange: (mode: ViewMode) => void;
	} = $props();
</script>

<div class="flex items-center gap-2">
	<div class="inline-flex rounded-lg border border-border-2 bg-card p-[2px]">
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors {viewMode ===
			'grid'
				? 'bg-muted text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
			onclick={() => onviewchange("grid")}
			title="Grid view"
			aria-label="Grid view"
			aria-pressed={viewMode === "grid"}
		>
			<LayoutGridIcon class="size-[15px]" strokeWidth={2} />
		</button>
		<button
			type="button"
			class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors {viewMode ===
			'list'
				? 'bg-muted text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
			onclick={() => onviewchange("list")}
			title="List view"
			aria-label="List view"
			aria-pressed={viewMode === "list"}
		>
			<ListIcon class="size-[15px]" strokeWidth={2} />
		</button>
	</div>
</div>
