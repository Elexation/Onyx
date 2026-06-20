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
			error = "Passwords don't match";
			return;
		}

		loading = true;
		try {
			await setup(password);
			await goto("/login");
		} catch (err) {
			error = err instanceof Error ? err.message : "Setup failed";
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4 py-10">
	<Card.Root class="w-full max-w-[460px]">
		<Card.Header>
			<Card.Title class="flex items-center gap-2.5 text-lg font-bold tracking-[-0.01em]">
				<svg
					width="28"
					height="28"
					viewBox="0 0 24 24"
					aria-hidden="true"
					class="block shrink-0"
				>
					<defs>
						<linearGradient id="onyx-setup-mark" x1="0" y1="0" x2="1" y2="1">
							<stop offset="0%" stop-color="oklch(0.82 0.12 245)" />
							<stop offset="100%" stop-color="oklch(0.55 0.13 245)" />
						</linearGradient>
					</defs>
					<path d="M12 2 L22 12 L12 22 L2 12 Z" fill="url(#onyx-setup-mark)" />
					<path d="M12 2 L22 12 L12 12 Z" fill="oklch(1 0 0 / 0.2)" />
					<path d="M2 12 L12 12 L12 22 Z" fill="oklch(0 0 0 / 0.24)" />
				</svg>
				Onyx
			</Card.Title>
			<Card.Action class="text-muted-foreground font-mono text-[11px]">setup</Card.Action>
			<Card.Description>
				First-run configuration. These settings can be changed later from the settings.
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<form onsubmit={handleSubmit} class="grid gap-4">
				<div class="grid gap-2">
					<Label for="username">Admin username</Label>
					<Input id="username" value="admin" disabled />
				</div>
				<div class="grid gap-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="At least 8 characters"
						bind:value={password}
						required
						autofocus
					/>
				</div>
				<div class="grid gap-2">
					<Label for="confirm">Confirm password</Label>
					<Input id="confirm" type="password" bind:value={confirm} required />
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button type="submit" size="lg" class="w-full" disabled={loading}>
					{loading ? "Creating admin…" : "Create admin"}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
