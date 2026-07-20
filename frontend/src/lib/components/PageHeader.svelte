<script lang="ts">
	import type { Snippet } from "svelte";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";

	type IconComponent = typeof Trash2Icon;

	let {
		icon: Icon,
		title,
		meta,
		subtitle,
		action,
	}: {
		icon: IconComponent;
		title: string;
		meta?: string;
		subtitle?: string;
		action?: Snippet;
	} = $props();
</script>

<div class="flex items-center gap-3">
	<div class="flex min-w-0 flex-1 flex-col">
		<div class="flex items-center gap-2">
			<Icon class="size-5 text-muted-foreground" strokeWidth={2} />
			<h1 class="text-lg font-bold tracking-[-0.01em]">{title}</h1>
			{#if meta}
				<span class="text-meta tabular-nums text-muted-foreground">{meta}</span>
			{/if}
		</div>
		{#if subtitle}
			<p class="mt-1 text-meta text-muted-foreground">{subtitle}</p>
		{/if}
	</div>
	{#if action}
		{@render action()}
	{/if}
</div>
