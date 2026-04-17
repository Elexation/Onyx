<script lang="ts">
	import AppHeader from "$lib/components/AppHeader.svelte";
	import Sidebar from "$lib/components/Sidebar.svelte";
	import UploadPanel from "$lib/components/UploadPanel.svelte";
	import { Toaster } from "$lib/components/ui/sonner/index.js";

	let { children } = $props();
</script>

<div class="flex h-screen flex-col">
	<AppHeader />
	<div class="flex min-h-0 flex-1">
		<Sidebar />
		<main class="flex-1 overflow-auto">
			{@render children()}
		</main>
	</div>
</div>
<UploadPanel />
<Toaster theme="dark" />
