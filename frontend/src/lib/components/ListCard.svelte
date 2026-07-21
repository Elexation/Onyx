<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		gridCols,
		header,
		children,
		headerRef = $bindable(null),
		class: className = "",
		headerClass = "",
	}: {
		gridCols: string;
		header: Snippet;
		children: Snippet;
		headerRef?: HTMLDivElement | null;
		class?: string;
		headerClass?: string;
	} = $props();
</script>

<div class="relative overflow-hidden rounded-xl border border-border bg-card {className}">
	<div
		bind:this={headerRef}
		class="hidden border-b border-border bg-list-header text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid {gridCols} md:gap-3 md:px-[14px] md:py-2.5 {headerClass}"
	>
		{@render header()}
	</div>
	{@render children()}
</div>
