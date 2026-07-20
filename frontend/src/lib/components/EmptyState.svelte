<script lang="ts">
	import PackageIcon from "@lucide/svelte/icons/package";
	type IconComponent = typeof PackageIcon;

	let {
		icon,
		title,
		description,
	}: {
		icon?: IconComponent;
		title: string;
		description?: string;
	} = $props();
</script>

<div class="flex flex-col items-center justify-center gap-3 py-16 text-center">
	{#if icon}
		{@const Icon = icon}
		<Icon class="size-10 text-muted-foreground opacity-30" strokeWidth={1.5} />
	{/if}
	<p class="text-[15px] text-foreground">{title}</p>
	{#if description}
		<p class="text-meta text-muted-foreground">{description}</p>
	{/if}
</div>
