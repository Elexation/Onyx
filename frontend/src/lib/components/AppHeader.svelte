<script lang="ts">
	import SearchIcon from "@lucide/svelte/icons/search";
	import XIcon from "@lucide/svelte/icons/x";
	import HeaderShell from "./HeaderShell.svelte";
	import SearchBar from "./SearchBar.svelte";
	import BrandMark from "./BrandMark.svelte";
	import UserChip from "./UserChip.svelte";
	import { Button } from "$lib/components/ui/button/index.js";

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

<HeaderShell
	hamburgerOpen={drawerOpen}
	onHamburgerToggle={() => (drawerOpen = !drawerOpen)}
	showHamburger={!mobileSearchOpen}
>
	{#if mobileSearchOpen}
		<Button variant="ghost" size="icon-header" class="text-muted-foreground md:hidden"
			aria-label="Close search"
			onclick={closeMobileSearch}
		>
			<XIcon class="size-5" strokeWidth={2} />
		</Button>
		<div class="flex-1 md:hidden">
			<SearchBar autoFocusKey={searchFocusKey} onescape={closeMobileSearch} />
		</div>
	{:else}
		<BrandMark />
	{/if}

	<div class="mx-auto hidden w-full max-w-[520px] flex-1 md:block">
		<SearchBar />
	</div>

	{#if !mobileSearchOpen}
		<div class="ml-auto flex items-center gap-1.5">
			<Button variant="ghost" size="icon-header" class="text-muted-foreground md:hidden"
				aria-label="Search"
				title="Search"
				onclick={openMobileSearch}
			>
				<SearchIcon class="size-[18px]" strokeWidth={2} />
			</Button>
			<UserChip />
		</div>
	{/if}
</HeaderShell>
