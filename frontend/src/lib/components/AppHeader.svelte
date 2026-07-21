<script lang="ts">
	import SearchIcon from "@lucide/svelte/icons/search";
	import XIcon from "@lucide/svelte/icons/x";
	import HeaderShell from "./HeaderShell.svelte";
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

<HeaderShell
	hamburgerOpen={drawerOpen}
	onHamburgerToggle={() => (drawerOpen = !drawerOpen)}
	showHamburger={!mobileSearchOpen}
>
	{#if mobileSearchOpen}
		<button
			type="button"
			class="inline-flex size-9 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground md:hidden"
			aria-label="Close search"
			onclick={closeMobileSearch}
		>
			<XIcon class="size-5" strokeWidth={2} />
		</button>
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
		</div>
	{/if}
</HeaderShell>
