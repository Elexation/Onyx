<script lang="ts">
	import type { Component, Snippet } from "svelte";

	let {
		icon: Icon,
		children,
	}: {
		icon: Component<{ class?: string }>;
		children: Snippet;
	} = $props();
</script>

<!-- Icon-row callout shared by the upload dialogs. -->
<div class="mt-1 flex items-center gap-3 rounded-lg border border-border bg-muted p-3">
	<Icon class="size-7 shrink-0 text-muted-foreground" />
	<div class="min-w-0">
		{@render children()}
	</div>
</div>
