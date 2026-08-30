<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/state";
	import AppHeader from "$lib/components/AppHeader.svelte";
	import Sidebar from "$lib/components/Sidebar.svelte";
	import MobileDrawer from "$lib/components/MobileDrawer.svelte";
	import UploadPanel from "$lib/components/UploadPanel.svelte";
	import AudioDock from "$lib/components/AudioDock.svelte";
	import { Toaster } from "$lib/components/ui/sonner/index.js";
	import { changes } from "$lib/changes";
	import { audioPlayer } from "$lib/stores/audioPlayer.svelte.js";
	import { uploadState } from "$lib/stores/upload.svelte.js";

	let { children } = $props();
	let drawerOpen = $state(false);

	onMount(() => {
		changes.start();
		return () => changes.stop();
	});

	// Close the mobile drawer on any route change. Sidebar's onNavigate prop
	// covers sidebar link clicks, but programmatic navigation (e.g. SearchBar
	// result click -> goto()) bypasses it — this effect catches those too.
	$effect(() => {
		page.url.pathname;
		drawerOpen = false;
	});

	// Warn before a full page reload / tab close while an upload is preparing or
	// in flight — a real unload tears down the module-level Uppy instance,
	// aborting active transfers and leaving folders partially uploaded. In-app
	// navigation is unaffected (the instance survives route changes).
	$effect(() => {
		const handler = (e: BeforeUnloadEvent) => {
			if (uploadState.preparing || uploadState.scanning || uploadState.activeCount > 0) {
				e.preventDefault();
				e.returnValue = "";
			}
		};
		window.addEventListener("beforeunload", handler);
		return () => window.removeEventListener("beforeunload", handler);
	});
</script>

<div class="flex h-screen flex-col">
	<AppHeader bind:drawerOpen />
	<div class="relative flex min-h-0 flex-1">
		<MobileDrawer bind:open={drawerOpen}>
			<Sidebar onNavigate={() => (drawerOpen = false)} />
		</MobileDrawer>
		<main class="min-w-0 flex-1 overflow-auto" style:padding-bottom={audioPlayer.visible ? '84px' : '0px'}>
			{@render children()}
		</main>
	</div>
</div>
<AudioDock />
<UploadPanel />
<Toaster theme="dark" />
