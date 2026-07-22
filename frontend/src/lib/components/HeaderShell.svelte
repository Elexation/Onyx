<script lang="ts">
	import type { Snippet } from "svelte";
	import MenuIcon from "@lucide/svelte/icons/menu";
	import { Button } from "$lib/components/ui/button/index.js";

	interface Props {
		hamburgerOpen?: boolean;
		onHamburgerToggle?: () => void;
		showHamburger?: boolean;
		children: Snippet;
	}
	let {
		hamburgerOpen = false,
		onHamburgerToggle,
		showHamburger = true,
		children,
	}: Props = $props();
</script>

<header
	class="relative z-30 flex h-16 shrink-0 items-center gap-3 border-b border-border bg-card px-4 max-md:h-14 max-md:gap-2 max-md:px-[14px]"
>
	{#if showHamburger}
		<Button variant="ghost" size="icon-header" class="text-muted-foreground md:hidden"
			aria-label="Open navigation"
			aria-expanded={hamburgerOpen}
			onclick={onHamburgerToggle}
		>
			<MenuIcon class="size-5" strokeWidth={2} />
		</Button>
	{/if}
	{@render children()}
</header>
