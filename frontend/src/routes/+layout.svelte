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

	function resolveRedirect(path: string, a: typeof auth): string | null {
		if (!a.checked) return null;
		if (a.firstRun && path !== "/setup") return "/setup";
		if (!a.firstRun && !a.authenticated && path !== "/login" && !path.startsWith("/s/")) return "/login";
		if (a.authenticated && (path === "/login" || path === "/setup")) return "/files";
		if (a.authenticated && path === "/") return "/files";
		return null;
	}

	const redirectTarget = $derived(resolveRedirect(page.url.pathname, auth));

	$effect(() => {
		if (redirectTarget) goto(redirectTarget);
	});
</script>

{#if auth.checked && !redirectTarget}
	{@render children()}
{/if}
