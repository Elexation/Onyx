<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { createToken } from "$lib/api/tokens.js";
	import { toast } from "svelte-sonner";
	import { Check, Copy, KeyRound } from "lucide-svelte";
	import type { PersonalAccessToken, TokenScope } from "$lib/types.js";

	let {
		open = $bindable(false),
		onCreated,
	}: {
		open: boolean;
		onCreated?: (token: PersonalAccessToken) => void;
	} = $props();

	let name = $state("");
	let scope = $state<string>("full");
	let noExpiry = $state(true);
	let expiryDate = $state(defaultExpiryDate());
	let submitting = $state(false);
	let createdToken = $state<PersonalAccessToken | null>(null);
	let closeEnabled = $state(false);
	let copied = $state(false);
	let tokenInput: HTMLInputElement | null = $state(null);

	const scopeOptions: { value: TokenScope; label: string; description: string }[] = [
		{ value: "read", label: "Read-only", description: "GET requests only — listing, download, preview" },
		{ value: "upload", label: "Upload + list", description: "Directory listing, upload, mkdir" },
		{ value: "full", label: "Full access", description: "All file operations (cannot manage tokens/settings)" },
	];

	const selectedScopeLabel = $derived(
		scopeOptions.find((o) => o.value === scope)?.label ?? "Full access"
	);

	function defaultExpiryDate(): string {
		const d = new Date();
		d.setDate(d.getDate() + 90);
		return d.toISOString().slice(0, 10);
	}

	$effect(() => {
		if (open) {
			name = "";
			scope = "full";
			noExpiry = true;
			expiryDate = defaultExpiryDate();
			submitting = false;
			createdToken = null;
			closeEnabled = false;
			copied = false;
		}
	});

	$effect(() => {
		if (createdToken && tokenInput) {
			tokenInput.focus();
			tokenInput.select();
			tokenInput.scrollLeft = 0;
			const timer = setTimeout(() => {
				closeEnabled = true;
			}, 2000);
			return () => clearTimeout(timer);
		}
	});

	async function submit() {
		const trimmed = name.trim();
		if (!trimmed) {
			toast.error("Name is required");
			return;
		}
		let expiresAt: number | null = null;
		if (!noExpiry) {
			const date = new Date(expiryDate + "T00:00:00Z");
			if (isNaN(date.getTime())) {
				toast.error("Invalid expiration date");
				return;
			}
			const seconds = Math.floor(date.getTime() / 1000);
			if (seconds <= Math.floor(Date.now() / 1000)) {
				toast.error("Expiration must be in the future");
				return;
			}
			expiresAt = seconds;
		}

		submitting = true;
		try {
			const tok = await createToken({ name: trimmed, scope: scope as TokenScope, expiresAt });
			createdToken = tok;
			onCreated?.(tok);
			toast.success("Token created");
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to create token");
		} finally {
			submitting = false;
		}
	}

	async function copyToken() {
		if (!createdToken?.token) return;
		await navigator.clipboard.writeText(createdToken.token);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	function scopeLabel(s: TokenScope): string {
		return scopeOptions.find((o) => o.value === s)?.label ?? s;
	}

	function formatExpiryDisplay(): string {
		if (!createdToken?.expiresAt) return "Never";
		return new Date(createdToken.expiresAt * 1000).toLocaleDateString(undefined, {
			month: "short",
			day: "numeric",
			year: "numeric",
		});
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		{#if createdToken}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					<KeyRound class="size-4" />
					Token Created
				</Dialog.Title>
				<Dialog.Description>
					Copy this token now. It will not be shown again.
				</Dialog.Description>
			</Dialog.Header>

			<div class="flex flex-col gap-4">
				<div class="flex gap-2">
					<Input
						bind:ref={tokenInput}
						value={createdToken.token ?? ""}
						readonly
						class="font-mono text-xs"
					/>
					<Button variant="outline" size="icon" onclick={copyToken} class="shrink-0">
						{#if copied}
							<Check class="size-4" />
						{:else}
							<Copy class="size-4" />
						{/if}
					</Button>
				</div>

				<div class="rounded-md border p-3 space-y-2 text-sm">
					<div class="flex justify-between">
						<span class="text-muted-foreground">Name</span>
						<span>{createdToken.name}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Scope</span>
						<span>{scopeLabel(createdToken.scope)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Expires</span>
						<span>{formatExpiryDisplay()}</span>
					</div>
				</div>
			</div>

			<Dialog.Footer>
				<Button onclick={() => (open = false)} disabled={!closeEnabled}>
					{closeEnabled ? "Close" : "Close (wait…)"}
				</Button>
			</Dialog.Footer>
		{:else}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					<KeyRound class="size-4" />
					Create Personal Access Token
				</Dialog.Title>
				<Dialog.Description>
					For authenticating scripts and automation against the Onyx API.
				</Dialog.Description>
			</Dialog.Header>

			<div class="flex flex-col gap-4">
				<div class="space-y-2">
					<Label for="token-name">Name</Label>
					<Input
						id="token-name"
						bind:value={name}
						placeholder="e.g. Backup script"
						maxlength={100}
					/>
				</div>

				<div class="space-y-2">
					<Label>Scope</Label>
					<Select.Root type="single" bind:value={scope}>
						<Select.Trigger>{selectedScopeLabel}</Select.Trigger>
						<Select.Content>
							{#each scopeOptions as opt}
								<Select.Item value={opt.value}>{opt.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
					<p class="text-xs text-muted-foreground">
						{scopeOptions.find((o) => o.value === scope)?.description}
					</p>
				</div>

				<div class="space-y-2">
					<div class="flex items-center justify-between">
						<Label for="token-expiry">Expiration</Label>
						<div class="flex items-center gap-2">
							<Label for="no-expiry" class="text-sm text-muted-foreground">No expiry</Label>
							<Switch id="no-expiry" bind:checked={noExpiry} />
						</div>
					</div>
					<Input
						id="token-expiry"
						type="date"
						bind:value={expiryDate}
						disabled={noExpiry}
					/>
				</div>
			</div>

			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)} disabled={submitting}>
					Cancel
				</Button>
				<Button onclick={submit} disabled={submitting || !name.trim()}>
					{submitting ? "Creating…" : "Create Token"}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
