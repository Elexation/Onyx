<script lang="ts">
	import { goto } from "$app/navigation";
	import { login } from "$lib/auth.svelte.js";
	import BrandMark from "$lib/components/BrandMark.svelte";
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
	<div class="flex w-full max-w-sm flex-col gap-6">
		<div class="flex justify-center">
			<BrandMark href="/login" />
		</div>
		<Card.Root class="w-full">
			<Card.Header>
				<Card.Title class="text-lg font-bold tracking-[-0.01em]">Sign in</Card.Title>
				<Card.Description>Enter your password to continue.</Card.Description>
			</Card.Header>
			<Card.Content>
				<form onsubmit={handleSubmit} class="grid gap-4">
					<div class="grid gap-2">
						<Label for="password">Password</Label>
						<Input
							id="password"
							type="password"
							bind:value={password}
							required
							autofocus
						/>
					</div>
					{#if error}
						<p class="text-sm text-destructive">{error}</p>
					{/if}
					<Button type="submit" size="lg" class="w-full" disabled={loading}>
						{loading ? "Signing in…" : "Sign In"}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	</div>
</div>
