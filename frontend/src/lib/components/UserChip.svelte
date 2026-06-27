<script lang="ts">
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { logout } from "$lib/auth.svelte.js";
	import { ChevronDown, LogOut, User } from "lucide-svelte";

	interface Props {
		name?: string;
	}
	let { name = "admin" }: Props = $props();
	const initial = $derived((name[0] ?? "a").toUpperCase());
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class="hidden items-center gap-2 rounded-full border border-transparent py-1 pr-2 pl-1 text-[13px] font-medium text-foreground transition-colors hover:bg-muted data-[state=open]:bg-muted lg:flex"
		aria-label="Signed in as {name}"
	>
		<span
			class="inline-flex size-7 items-center justify-center rounded-full bg-accent-brand-dim font-mono text-[11px] font-semibold text-accent-brand"
			aria-hidden="true"
		>
			{initial}
		</span>
		<span class="font-mono">{name}</span>
		<ChevronDown class="size-3.5 text-muted-foreground" strokeWidth={2} />
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="w-auto min-w-[13rem]">
		<DropdownMenu.Label
			class="flex items-center gap-1.5 px-1.5 py-1 text-sm font-normal text-foreground"
		>
			<User class="size-4" />
			<span>Signed in as {name}</span>
		</DropdownMenu.Label>
		<DropdownMenu.Separator />
		<DropdownMenu.Item variant="destructive" onclick={logout}>
			<LogOut />
			Sign out
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>
