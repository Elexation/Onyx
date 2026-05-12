<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/state";
	import { FolderOpen, Link, Trash2, Settings } from "lucide-svelte";
	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";

	const links = [
		{ href: "/files", label: "Files", icon: FolderOpen },
		{ href: "/shares", label: "Shares", icon: Link },
		{ href: "/trash", label: "Trash", icon: Trash2 },
		{ href: "/settings", label: "Settings", icon: Settings },
	];

	onMount(() => {
		trashCount.startPolling();
		sharesEnabled.refresh();
		return () => trashCount.stopPolling();
	});
</script>

<nav class="flex w-48 shrink-0 flex-col gap-1 border-r border-border bg-card p-3">
	{#each links as link}
		{@const disabled = link.href === "/shares" && !sharesEnabled.enabled}
		{#if disabled}
			<span
				class="flex cursor-not-allowed items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground/40"
			>
				<link.icon class="size-4" />
				{link.label}
			</span>
		{:else}
			<a
				href={link.href}
				class="flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent {page.url.pathname.startsWith(link.href) ? 'bg-accent text-accent-foreground' : 'text-muted-foreground'}"
			>
				<link.icon class="size-4" />
				{link.label}
				{#if link.href === "/trash" && trashCount.count > 0}
					<span class="ml-auto rounded-full bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
						{trashCount.count}
					</span>
				{/if}
			</a>
		{/if}
	{/each}
</nav>
