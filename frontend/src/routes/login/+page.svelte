<script lang="ts">
	import { goto } from "$app/navigation";
	import { login } from "$lib/auth.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";

	let password = $state("");
	let error = $state("");
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = "";
		loading = true;

		try {
			await login(password);
			await goto("/files");
		} catch (err) {
			error = err instanceof Error ? err.message : "Login failed";
			password = "";
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4 py-10">
	<Card.Root class="w-full max-w-sm">
		<Card.Header class="items-center gap-2 text-center">
			<svg
				width="40"
				height="40"
				viewBox="0 0 24 24"
				aria-hidden="true"
				class="mx-auto block shrink-0"
			>
				<defs>
					<linearGradient id="onyx-login-mark" x1="0" y1="0" x2="1" y2="1">
						<stop offset="0%" stop-color="oklch(0.82 0.12 245)" />
						<stop offset="100%" stop-color="oklch(0.55 0.13 245)" />
					</linearGradient>
				</defs>
				<path d="M12 2 L22 12 L12 22 L2 12 Z" fill="url(#onyx-login-mark)" />
				<path d="M12 2 L22 12 L12 12 Z" fill="oklch(1 0 0 / 0.2)" />
				<path d="M2 12 L12 12 L12 22 Z" fill="oklch(0 0 0 / 0.24)" />
			</svg>
			<Card.Title class="text-xl font-bold tracking-[-0.01em]">Onyx</Card.Title>
			<Card.Description>Sign in to continue</Card.Description>
		</Card.Header>
		<Card.Content>
			<form onsubmit={handleSubmit} class="grid gap-4">
				<div class="grid gap-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="Enter admin password"
						bind:value={password}
						required
						autofocus
					/>
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button type="submit" size="lg" class="w-full" disabled={loading}>
					{loading ? "Signing in…" : "Sign in"}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
