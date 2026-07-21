<script lang="ts">
	import { page } from "$app/state";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import BrandMark from "$lib/components/BrandMark.svelte";
	import HeaderShell from "$lib/components/HeaderShell.svelte";
	import DocsToc from "$lib/components/DocsToc.svelte";
	import { docsDrawer } from "$lib/stores/docsDrawer.svelte.js";
	import { viewport } from "$lib/stores/viewport.svelte.js";

	let { children } = $props();

	$effect(() => {
		page.url.pathname;
		docsDrawer.open = false;
	});

	$effect(() => {
		if (!viewport.isMobile) docsDrawer.open = false;
	});
</script>

<div class="flex h-screen flex-col">
	<HeaderShell
		hamburgerOpen={docsDrawer.open}
		onHamburgerToggle={() => (docsDrawer.open = !docsDrawer.open)}
	>
		<BrandMark />
		<a
			href="/files"
			class="ml-auto flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
		>
			<ArrowLeftIcon class="size-4" />
			Back to files
		</a>
	</HeaderShell>

	<div class="relative flex min-h-0 flex-1">
		<DocsToc />
		<main id="docs-scroll" class="min-w-0 flex-1 overflow-auto">
			{@render children()}
		</main>
	</div>
</div>
