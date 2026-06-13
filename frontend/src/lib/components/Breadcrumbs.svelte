<script lang="ts">
	import { Home, ChevronRight } from "lucide-svelte";
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
		const parts = path.split("/").filter(Boolean);
		return parts.map((name, i) => ({
			name,
			href: "/files/" + parts.slice(0, i + 1).join("/"),
			dirPath: parts.slice(0, i + 1).join("/"),
			isLast: i === parts.length - 1,
		}));
	});

	const atRoot = $derived(segments.length === 0);

	function noop() {}
</script>

<nav class="flex min-w-0 flex-wrap items-center gap-1 text-[15px]">
	<a
		href="/files"
		class="inline-flex items-center gap-1.5 rounded-md px-2 py-1 font-medium transition-colors hover:bg-muted hover:text-foreground {atRoot
			? 'text-foreground'
			: 'text-muted-foreground'}"
		title="Files root"
		use:droppable={{ path: "", ondrop: ondrop ?? noop }}
	>
		<Home class="size-[15px]" strokeWidth={2} />
		<span>Home</span>
	</a>
	{#each segments as segment}
		<span class="inline-flex text-[oklch(0.45_0_0)]">
			<ChevronRight class="size-3.5" strokeWidth={2} />
		</span>
		{#if segment.isLast}
			<span
				class="inline-flex items-center rounded-md px-2 py-1 font-semibold text-foreground"
			>
				{segment.name}
			</span>
		{:else}
			<a
				href={segment.href}
				class="inline-flex items-center rounded-md px-2 py-1 font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				use:droppable={{ path: segment.dirPath, ondrop: ondrop ?? noop }}
			>
				{segment.name}
			</a>
		{/if}
	{/each}
</nav>
