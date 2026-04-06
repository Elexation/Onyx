<script lang="ts">
	import "../app.css";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { auth, checkStatus } from "$lib/auth.svelte.js";
	import { onMount } from "svelte";

	let { children } = $props();

	onMount(async () => {
		try {
			await checkStatus();
		} catch {
			auth.checked = true;
		}
	});

	$effect(() => {
		if (!auth.checked) return;
		const path = page.url.pathname;

		if (auth.firstRun && path !== "/setup") {
			goto("/setup");
		} else if (!auth.firstRun && !auth.authenticated && path !== "/login") {
			goto("/login");
		} else if (auth.authenticated && (path === "/login" || path === "/setup")) {
			goto("/files");
		} else if (auth.authenticated && path === "/") {
			goto("/files");
		}
	});
</script>

{#if auth.checked}
	{@render children()}
{/if}
