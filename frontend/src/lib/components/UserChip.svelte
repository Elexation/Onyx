<script lang="ts">
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { goto } from "$app/navigation";
	import { logout } from "$lib/auth.svelte.js";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import FileTextIcon from "@lucide/svelte/icons/file-text";
	import LogOutIcon from "@lucide/svelte/icons/log-out";
	import UserIcon from "@lucide/svelte/icons/user";

	interface Props {
		name?: string;
	}
	let { name = "admin" }: Props = $props();
	const initial = $derived((name[0] ?? "a").toUpperCase());
</script>

{#snippet menuItems()}
	<DropdownMenu.Label
		class="flex items-center gap-1.5 px-1.5 py-1 text-sm font-normal text-foreground"
	>
		<UserIcon class="size-4" />
		<span>Signed in as {name}</span>
	</DropdownMenu.Label>
	<DropdownMenu.Item onclick={() => goto('/docs/api')}>
		<FileTextIcon />
		API Reference
	</DropdownMenu.Item>
	<DropdownMenu.Separator />
	<DropdownMenu.Item variant="destructive" onclick={logout}>
		<LogOutIcon />
		Sign out
	</DropdownMenu.Item>
{/snippet}

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class="hidden items-center gap-2 rounded-full border border-transparent py-1 pr-2 pl-1 text-meta font-medium text-foreground outline-none transition-colors hover:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring/50 data-[state=open]:bg-muted lg:flex"
		aria-label="Signed in as {name}"
	>
		<span
			class="inline-flex size-7 items-center justify-center rounded-full bg-accent-brand-dim font-mono text-[11px] font-semibold text-accent-brand"
			aria-hidden="true"
		>
			{initial}
		</span>
		<span class="font-mono">{name}</span>
		<ChevronDownIcon class="size-3.5 text-muted-foreground" strokeWidth={2} />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="w-auto min-w-[13rem]">
		{@render menuItems()}
	</DropdownMenu.Content>
</DropdownMenu.Root>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class="inline-flex size-9 items-center justify-center rounded-full bg-accent-brand-dim font-mono text-meta font-semibold text-accent-brand outline-none transition-colors hover:bg-accent-brand/20 focus-visible:ring-[3px] focus-visible:ring-ring/50 data-[state=open]:bg-accent-brand/20 lg:hidden"
		aria-label="Signed in as {name}"
	>
		{initial}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="w-auto min-w-[13rem]">
		{@render menuItems()}
	</DropdownMenu.Content>
</DropdownMenu.Root>
