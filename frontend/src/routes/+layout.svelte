<script lang="ts">
	import "../app.css";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { auth, checkStatus } from "$lib/auth.svelte.js";
	import { onMount } from "svelte";
	import Button from "$lib/components/ui/button/button.svelte";

	let { children } = $props();

	let bootstrapError = $state(false);
	let retrying = $state(false);

	onMount(async () => {
		try {
			await checkStatus();
		} catch {
			bootstrapError = true;
		}
	});

	async function retryBootstrap() {
		retrying = true;
		try {
			await checkStatus();
			bootstrapError = false;
		} catch {
			bootstrapError = true;
		} finally {
			retrying = false;
		}
	}

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

{#if bootstrapError}
	<div class="flex h-screen items-center justify-center bg-background">
		<div class="text-center">
			<p class="text-lg font-semibold text-foreground">Can't reach the server</p>
			<p class="mt-2 text-sm text-muted-foreground">Check that Onyx is running and try again.</p>
			<Button class="mt-4" disabled={retrying} onclick={retryBootstrap}>
				{retrying ? "Retrying…" : "Retry"}
			</Button>
		</div>
	</div>
{:else if auth.checked && !redirectTarget}
	{@render children()}
{/if}
