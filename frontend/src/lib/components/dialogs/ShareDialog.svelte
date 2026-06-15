<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { createShare, getShareByPath, deleteShare } from "$lib/api/shares.js";
	import { toast } from "svelte-sonner";
	import { Check, Copy, Link, Loader2 } from "lucide-svelte";
	import type { ShareLink } from "$lib/types.js";

	let {
		open = $bindable(false),
		path,
		isDir,
	}: {
		open: boolean;
		path: string;
		isDir: boolean;
	} = $props();

	let expiresIn = $state("168h");
	let usePassword = $state(false);
	let password = $state("");
	let submitting = $state(false);
	let shareUrl = $state("");
	let copied = $state(false);
	let loading = $state(false);
	let existing = $state<ShareLink | null>(null);
	let revoking = $state(false);
	let showCreateForm = $state(false);
	let createError = $state("");

	const expiryOptions = [
		{ value: "1h", label: "1 hour" },
		{ value: "24h", label: "1 day" },
		{ value: "168h", label: "7 days" },
		{ value: "720h", label: "30 days" },
		{ value: "2160h", label: "90 days" },
	];

	const selectedLabel = $derived(
		expiryOptions.find((o) => o.value === expiresIn)?.label ?? "7 days"
	);

	$effect(() => {
		if (open) {
			expiresIn = "168h";
			usePassword = false;
			password = "";
			shareUrl = "";
			copied = false;
			existing = null;
			showCreateForm = false;
			createError = "";
			loading = true;
			getShareByPath(path)
				.then((link) => {
					existing = link;
				})
				.catch(() => {
					existing = null;
				})
				.finally(() => {
					loading = false;
				});
		}
	});

	async function submit() {
		submitting = true;
		try {
			const result = await createShare({
				path,
				isDir,
				expiresIn,
				password: usePassword ? password : undefined,
			});
			shareUrl = `${window.location.origin}/s/${result.token}`;
			existing = null;
			showCreateForm = false;
			toast.success("Share link created");
		} catch (e) {
			const msg = e instanceof Error ? e.message : "Failed to create share";
			createError = msg;
			toast.error(msg);
		} finally {
			submitting = false;
		}
	}

	async function revoke() {
		if (!existing) return;
		revoking = true;
		try {
			await deleteShare(existing.id);
			existing = null;
			showCreateForm = true;
			toast.success("Share link revoked");
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to revoke share");
		} finally {
			revoking = false;
		}
	}

	async function copyUrl() {
		await navigator.clipboard.writeText(shareUrl);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	function formatDate(unix: number): string {
		return new Date(unix * 1000).toLocaleDateString(undefined, {
			month: "short",
			day: "numeric",
			year: "numeric",
			hour: "numeric",
			minute: "2-digit",
		});
	}

	function formatExpiry(link: ShareLink): string {
		if (!link.expiresAt) return "Never";
		const now = Date.now() / 1000;
		const remaining = link.expiresAt - now;
		if (remaining <= 0) return "Expired";
		if (remaining < 3600) return `${Math.ceil(remaining / 60)}m remaining`;
		if (remaining < 86400) return `${Math.ceil(remaining / 3600)}h remaining`;
		return `${Math.ceil(remaining / 86400)}d remaining`;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content
		class="sm:max-w-md"
		escapeKeydownBehavior={shareUrl ? "ignore" : "close"}
		interactOutsideBehavior={shareUrl ? "ignore" : "close"}
		showCloseButton={!shareUrl}
		onInteractOutside={(e: Event) => { if (shareUrl) e.preventDefault(); }}
	>
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<Link class="size-4" />
				Share
			</Dialog.Title>
			<Dialog.Description class="truncate">
				{path}
			</Dialog.Description>
		</Dialog.Header>

		{#if loading}
			<div class="flex items-center justify-center py-6">
				<Loader2 class="size-5 animate-spin text-muted-foreground" />
			</div>
		{:else if shareUrl}
			<div class="flex flex-col gap-3">
				<Label>Share URL</Label>
				<div class="flex gap-2">
					<Input value={shareUrl} readonly class="font-mono text-xs" />
					<Button variant="outline" size="icon" onclick={copyUrl} class="shrink-0">
						{#if copied}
							<Check class="size-4" />
						{:else}
							<Copy class="size-4" />
						{/if}
					</Button>
				</div>
				<p class="text-xs text-muted-foreground">
					This link will only be shown once. Copy it now.
				</p>
			</div>
			<Dialog.Footer>
				<Button onclick={() => (open = false)}>Done</Button>
			</Dialog.Footer>
		{:else if existing && !showCreateForm}
			<div class="flex flex-col gap-4">
				<p class="text-sm text-muted-foreground">
					This {isDir ? "folder" : "file"} already has an active share link.
				</p>
				<div class="rounded-lg border border-border bg-background p-3 space-y-2 text-sm">
					<div class="flex justify-between">
						<span class="text-muted-foreground">Created</span>
						<span class="font-mono text-[13px]">{formatDate(existing.createdAt)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Expires</span>
						<span class="font-mono text-[13px]">{formatExpiry(existing)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Downloads</span>
						<span class="font-mono text-[13px]">{existing.downloadCount}</span>
					</div>
					{#if existing.hasPassword}
						<div class="flex justify-between">
							<span class="text-muted-foreground">Password</span>
							<span class="font-mono text-[13px]">Yes</span>
						</div>
					{/if}
				</div>
				<p class="text-xs text-muted-foreground">
					Lost the link? Revoke it and create a new one.
				</p>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)}>Close</Button>
				<Button variant="destructive" onclick={revoke} disabled={revoking}>
					{revoking ? "Revoking…" : "Revoke & Create New"}
				</Button>
			</Dialog.Footer>
		{:else}
			<div class="flex flex-col gap-4">
				{#if createError}
					<p class="text-sm text-destructive">{createError}</p>
				{/if}
				<div class="flex flex-col gap-2">
					<Label>Expiration</Label>
					<Select.Root type="single" bind:value={expiresIn}>
						<Select.Trigger>
							{selectedLabel}
						</Select.Trigger>
						<Select.Content>
							{#each expiryOptions as opt}
								<Select.Item value={opt.value}>{opt.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>

				<div class="flex items-center justify-between">
					<Label>Password protection</Label>
					<Switch bind:checked={usePassword} />
				</div>

				{#if usePassword}
					<Input
						type="password"
						bind:value={password}
						placeholder="Enter password"
					/>
				{/if}
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
				<Button onclick={submit} disabled={submitting || (usePassword && !password)}>
					Create Link
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
