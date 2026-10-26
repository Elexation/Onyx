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

	// Everything floating clears the dock through this one variable
	$effect(() => {
		const root = document.documentElement;
		root.style.setProperty("--dock-inset", audioPlayer.visible ? "84px" : "0px");
		return () => root.style.removeProperty("--dock-inset");
	});

	// Close drawer on any route change, incl. goto()
	$effect(() => {
		page.url.pathname;
		drawerOpen = false;
	});

	// Warn on unload while uploads run, since unload kills Uppy
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
		<main class="min-w-0 flex-1 overflow-auto pb-(--dock-inset)">
			{@render children()}
		</main>
	</div>
</div>
<AudioDock />
<UploadPanel />
<!-- Toasts sit above the audio dock, the upload panel and (on phones) the FAB -->
<Toaster
	offset={{ bottom: "calc(var(--dock-inset) + 24px + var(--upload-panel-h))" }}
	mobileOffset={{ bottom: "calc(var(--dock-inset) + var(--fab-clearance) + var(--upload-panel-h))" }}
/>
