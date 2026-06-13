<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/state";
	import { FolderOpen, Link as LinkIcon, Trash2, Settings } from "lucide-svelte";
	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { versioningEnabled } from "$lib/stores/versioningEnabled.svelte.js";
	import StorageMeter from "./StorageMeter.svelte";

	interface Props {
		onNavigate?: () => void;
	}
	let { onNavigate }: Props = $props();

	const links = [
		{ href: "/files", label: "Files", icon: FolderOpen },
		{ href: "/shares", label: "Shares", icon: LinkIcon },
		{ href: "/trash", label: "Trash", icon: Trash2 },
		{ href: "/settings", label: "Settings", icon: Settings },
	];

	onMount(() => {
		trashCount.startPolling();
		sharesEnabled.refresh();
		versioningEnabled.refresh();
		return () => trashCount.stopPolling();
	});

	function activeFor(href: string) {
		return page.url.pathname === href || page.url.pathname.startsWith(href + "/");
	}
</script>

<aside
	class="flex h-full w-[220px] shrink-0 flex-col gap-[14px] overflow-auto border-r border-border bg-card p-[14px] max-md:w-[280px]"
>
	<nav class="flex flex-col gap-[2px]">
		{#each links as link}
			{@const disabled = link.href === "/shares" && !sharesEnabled.enabled}
			{@const active = activeFor(link.href)}
			{#if disabled}
				<span
					class="flex min-h-[38px] cursor-not-allowed items-center gap-[10px] rounded-lg px-3 py-[9px] text-sm font-medium text-muted-foreground/40"
				>
					<link.icon class="size-[17px]" strokeWidth={2} />
					<span class="flex-1">{link.label}</span>
				</span>
			{:else}
				<a
					href={link.href}
					onclick={() => onNavigate?.()}
					class="flex min-h-[38px] items-center gap-[10px] rounded-lg px-3 py-[9px] text-sm font-medium transition-colors hover:bg-muted {active
						? 'bg-muted text-foreground'
						: 'text-[oklch(0.82_0_0)]'}"
				>
					<link.icon class="size-[17px]" strokeWidth={2} />
					<span class="flex-1">{link.label}</span>
					{#if link.href === "/trash" && trashCount.count > 0}
						<span
							class="rounded-[5px] px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] text-muted-foreground {active
								? 'bg-background'
								: 'bg-muted'}"
						>
							{trashCount.count}
						</span>
					{/if}
				</a>
			{/if}
		{/each}
	</nav>

	<StorageMeter />
</aside>
