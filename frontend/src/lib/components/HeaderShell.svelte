<script lang="ts">
	import type { Snippet } from "svelte";
	import MenuIcon from "@lucide/svelte/icons/menu";

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
		<button
			type="button"
			class="inline-flex size-9 min-h-11 min-w-11 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground md:hidden"
			aria-label="Open navigation"
			aria-expanded={hamburgerOpen}
			onclick={onHamburgerToggle}
		>
			<MenuIcon class="size-5" strokeWidth={2} />
		</button>
	{/if}
	{@render children()}
</header>
