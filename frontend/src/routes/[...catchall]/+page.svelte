<script lang="ts">
	import { page } from "$app/state";
	import { Button } from "$lib/components/ui/button";

	const subhead = $derived(
		page.url.pathname.startsWith("/s/")
			? "The share link may have expired or been revoked."
			: "That page doesn't exist. It may have been moved or deleted."
	);
</script>

<div class="flex min-h-screen items-center justify-center p-6 text-center">
	<div class="flex flex-col items-center">
		<div
			class="text-border-2 font-mono text-9xl leading-none font-bold tracking-[-0.02em]"
			aria-hidden="true"
		>
			404
		</div>
		<h1 class="mt-6 text-xl font-semibold tracking-[-0.01em]">Page not found</h1>
		<p class="mt-2 max-w-[45ch] text-sm text-muted-foreground">{subhead}</p>
		<Button href="/" variant="outline" class="mt-6">Back to home</Button>
	</div>
</div>
