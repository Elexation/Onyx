<script lang="ts">
	import { page } from "$app/state";
	import { FolderOpen, Trash2, Settings } from "lucide-svelte";

	const links = [
		{ href: "/files", label: "Files", icon: FolderOpen, disabled: false },
		{ href: "/trash", label: "Trash", icon: Trash2, disabled: true },
		{ href: "/settings", label: "Settings", icon: Settings, disabled: true },
	];
</script>

<nav class="flex w-48 shrink-0 flex-col gap-1 border-r border-border bg-card p-3">
	{#each links as link}
		{#if link.disabled}
			<span class="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground opacity-50">
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
			</a>
		{/if}
	{/each}
</nav>
