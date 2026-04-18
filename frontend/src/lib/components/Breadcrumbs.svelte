<script lang="ts">
	import { Home } from "lucide-svelte";
	import { droppable } from "$lib/actions/droppable.js";

	let {
		path = "",
		ondrop,
	}: {
		path?: string;
		ondrop?: (paths: string[], destination: string) => void;
	} = $props();

	const segments = $derived.by(() => {
		if (!path) return [];
		return path.split("/").filter(Boolean).map((name, i, arr) => ({
			name,
			href: "/files/" + arr.slice(0, i + 1).join("/"),
			dirPath: arr.slice(0, i + 1).join("/"),
		}));
	});

	function noop() {}
</script>

<nav class="flex items-center gap-1.5 text-sm">
	<a
		href="/files"
		class="flex items-center text-muted-foreground transition-colors hover:text-foreground"
		title="Files root"
		use:droppable={{ path: "", ondrop: ondrop ?? noop }}
	>
		<Home class="size-4" />
	</a>
	{#each segments as segment}
		<span class="text-muted-foreground">›</span>
		<a
			href={segment.href}
			class="text-muted-foreground transition-colors hover:text-foreground"
			use:droppable={{ path: segment.dirPath, ondrop: ondrop ?? noop }}
		>
			{segment.name}
		</a>
	{/each}
</nav>
