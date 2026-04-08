<script lang="ts">
	import { Home } from "lucide-svelte";

	let { path = "" }: { path?: string } = $props();

	const segments = $derived.by(() => {
		if (!path) return [];
		return path.split("/").filter(Boolean).map((name, i, arr) => ({
			name,
			href: "/files/" + arr.slice(0, i + 1).join("/"),
		}));
	});
</script>

<nav class="flex items-center gap-1.5 text-sm">
	<a href="/files" class="flex items-center text-muted-foreground transition-colors hover:text-foreground" title="Files root">
		<Home class="size-4" />
	</a>
	{#each segments as segment}
		<span class="text-muted-foreground">›</span>
		<a href={segment.href} class="text-muted-foreground transition-colors hover:text-foreground">
			{segment.name}
		</a>
	{/each}
</nav>
