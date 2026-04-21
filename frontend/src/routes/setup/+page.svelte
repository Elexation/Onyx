<script lang="ts">
	import { goto } from "$app/navigation";
	import { setup } from "$lib/auth.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";

	const MIN_PASSWORD_LENGTH = 8;

	let password = $state("");
	let confirm = $state("");
	let error = $state("");
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = "";

		if (password.length < MIN_PASSWORD_LENGTH) {
			error = `Password must be at least ${MIN_PASSWORD_LENGTH} characters`;
			return;
		}
		if (password !== confirm) {
			error = "Passwords do not match";
			return;
		}

		loading = true;
		try {
			await setup(password);
			await goto("/files");
		} catch (err) {
			error = err instanceof Error ? err.message : "Setup failed";
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<Card.Root class="w-full max-w-sm">
		<Card.Header>
			<Card.Title class="text-2xl">Set Admin Password</Card.Title>
			<Card.Description>Create a password to secure your Onyx instance.</Card.Description>
		</Card.Header>
		<Card.Content>
			<form onsubmit={handleSubmit} class="grid gap-4">
				<div class="grid gap-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="Minimum {MIN_PASSWORD_LENGTH} characters"
						bind:value={password}
						required
						autofocus
					/>
				</div>
				<div class="grid gap-2">
					<Label for="confirm">Confirm Password</Label>
					<Input
						id="confirm"
						type="password"
						placeholder="Repeat your password"
						bind:value={confirm}
						required
					/>
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button type="submit" class="w-full" disabled={loading}>
					{loading ? "Setting up…" : "Create Admin"}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
