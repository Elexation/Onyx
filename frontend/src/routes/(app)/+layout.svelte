<script lang="ts">
	import { page } from "$app/state";
	import AppHeader from "$lib/components/AppHeader.svelte";
	import Sidebar from "$lib/components/Sidebar.svelte";
	import MobileDrawer from "$lib/components/MobileDrawer.svelte";
	import UploadPanel from "$lib/components/UploadPanel.svelte";
	import { Toaster } from "$lib/components/ui/sonner/index.js";

	let { children } = $props();
	let drawerOpen = $state(false);

	// Close the mobile drawer on any route change. Sidebar's onNavigate prop
	// covers sidebar link clicks, but programmatic navigation (e.g. SearchBar
	// result click -> goto()) bypasses it — this effect catches those too.
	$effect(() => {
		page.url.pathname;
		drawerOpen = false;
	});
</script>

<div class="flex h-screen flex-col">
	<AppHeader bind:drawerOpen />
	<div class="relative flex min-h-0 flex-1">
		<MobileDrawer bind:open={drawerOpen}>
			<Sidebar onNavigate={() => (drawerOpen = false)} />
		</MobileDrawer>
		<main class="min-w-0 flex-1 overflow-auto">
			{@render children()}
		</main>
	</div>
</div>
<UploadPanel />
<Toaster theme="dark" />
