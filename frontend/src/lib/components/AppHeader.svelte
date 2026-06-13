<script lang="ts">
	import { logout } from "$lib/auth.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { LogOut, Menu, Search as SearchIcon, X } from "lucide-svelte";
	import SearchBar from "./SearchBar.svelte";
	import BrandMark from "./BrandMark.svelte";
	import UserChip from "./UserChip.svelte";

	interface Props {
		drawerOpen?: boolean;
	}
	let { drawerOpen = $bindable(false) }: Props = $props();

	let mobileSearchOpen = $state(false);
	let searchFocusKey = $state(0);

	function openMobileSearch() {
		mobileSearchOpen = true;
		searchFocusKey += 1;
	}

	function closeMobileSearch() {
		mobileSearchOpen = false;
	}
</script>

<header
	class="relative z-30 flex h-16 shrink-0 items-center gap-3 border-b border-border bg-card px-4 max-md:h-14 max-md:gap-2 max-md:px-[14px]"
>
	{#if mobileSearchOpen}
		<!-- Mobile search expanded: X close + full-width SearchBar -->
		<button
			type="button"
			class="inline-flex size-9 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground md:hidden"
			aria-label="Close search"
			onclick={closeMobileSearch}
		>
			<X class="size-5" strokeWidth={2} />
		</button>
		<div class="flex-1 md:hidden">
			<SearchBar autoFocusKey={searchFocusKey} onescape={closeMobileSearch} />
		</div>
	{:else}
		<button
			type="button"
			class="inline-flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground md:hidden"
			aria-label="Open navigation"
			aria-expanded={drawerOpen}
			onclick={() => (drawerOpen = !drawerOpen)}
		>
			<Menu class="size-5" strokeWidth={2} />
		</button>

		<BrandMark />
	{/if}

	<!-- Desktop SearchBar (always in flow on desktop) -->
	<div class="mx-auto hidden w-full max-w-[520px] flex-1 md:block">
		<SearchBar />
	</div>

	{#if !mobileSearchOpen}
		<div class="ml-auto flex items-center gap-1.5">
			<button
				type="button"
				class="inline-flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground md:hidden"
				aria-label="Search"
				title="Search"
				onclick={openMobileSearch}
			>
				<SearchIcon class="size-[18px]" strokeWidth={2} />
			</button>
			<UserChip />
			<Button variant="ghost" size="icon-sm" onclick={logout} title="Sign out" aria-label="Sign out">
				<LogOut class="size-[18px]" strokeWidth={2} />
			</Button>
		</div>
	{/if}
</header>
