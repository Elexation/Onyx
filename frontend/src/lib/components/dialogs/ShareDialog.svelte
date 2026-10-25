<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { createShare, getShareByPath, deleteShare } from "$lib/api/shares.js";
	import { sharedPaths } from "$lib/stores/sharedPaths.svelte.js";
	import { toast } from "svelte-sonner";
	import CheckIcon from "@lucide/svelte/icons/check";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import LinkIcon from "@lucide/svelte/icons/link";
	import LoadingState from "$lib/components/LoadingState.svelte";
	import type { ShareLink } from "$lib/types.js";
	import { copyToClipboard } from "$lib/utils/clipboard.js";
	import { formatAbsoluteDateTime, formatRemainingLong } from "$lib/utils/format.js";

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
	let copiedTimer: ReturnType<typeof setTimeout>;
	let loading = $state(false);
	let existing = $state<ShareLink | null>(null);
	let showCreateForm = $state(false);
	let urlInputRef = $state<HTMLInputElement | null>(null);
	let closeEnabled = $state(false);

	const expiryOptions = [
		{ value: "1h", label: "1 hour" },
		{ value: "24h", label: "1 day" },
		{ value: "168h", label: "7 days" },
		{ value: "720h", label: "30 days" },
		{ value: "2160h", label: "90 days" },
		{ value: "never", label: "Never" },
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
			clearTimeout(copiedTimer);
			existing = null;
			showCreateForm = false;
			closeEnabled = false;
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

	$effect(() => {
		if (shareUrl) {
			const timer = setTimeout(() => {
				closeEnabled = true;
			}, 2000);
			return () => clearTimeout(timer);
		}
	});

	async function submit() {
		submitting = true;
		let revoked = false;
		try {
			// One link per path, so Replace must revoke the old link first
			if (existing) {
				await deleteShare(existing.id);
				existing = null;
				sharedPaths.remove(path);
				revoked = true;
			}
			const result = await createShare({
				path,
				isDir,
				expiresIn: expiresIn === "never" ? undefined : expiresIn,
				password: usePassword ? password : undefined,
			});
			shareUrl = `${window.location.origin}/s/${result.token}`;
			existing = null;
			showCreateForm = false;
			sharedPaths.add(path);
			toast.success("Share link created");
		} catch (e) {
			const reason = e instanceof Error ? e.message : "Failed to create share";
			toast.error(revoked ? `The old link was revoked, but a new one couldn't be created: ${reason}` : reason);
		} finally {
			submitting = false;
		}
	}

	async function copyUrl() {
		try {
			await copyToClipboard(shareUrl, urlInputRef ?? undefined);
			copied = true;
			clearTimeout(copiedTimer);
			copiedTimer = setTimeout(() => (copied = false), 2000);
		} catch {
			toast.error("Couldn't copy. Select the link and copy manually");
		}
	}

</script>

<Dialog.Root bind:open>
	<Dialog.Content
		class="sm:max-w-md"
		escapeKeydownBehavior={shareUrl || submitting ? "ignore" : "close"}
		interactOutsideBehavior={shareUrl || submitting ? "ignore" : "close"}
		showCloseButton={!shareUrl && !submitting}
		onInteractOutside={(e: Event) => { if (shareUrl || submitting) e.preventDefault(); }}
	>
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<LinkIcon class="size-4" />
				Share
			</Dialog.Title>
			<Dialog.Description class="truncate font-mono text-xs">
				{path}
			</Dialog.Description>
		</Dialog.Header>

		<div class="min-h-[200px]">
		{#if loading}
			<LoadingState compact />
		{:else if shareUrl}
			<div class="flex flex-col gap-3">
				<Label>Share URL</Label>
				<div class="flex gap-2">
					<Input bind:ref={urlInputRef} value={shareUrl} readonly class="font-mono text-xs" />
					<Button variant="outline" size="icon" onclick={copyUrl} class="shrink-0" aria-label={copied ? "Copied" : "Copy link"}>
						<span class="relative block size-4 overflow-hidden">
							<CopyIcon
								class="absolute inset-0 size-4 transition-all duration-150 ease-out {copied ? '-translate-y-2 opacity-0' : 'translate-y-0 opacity-100'}"
							/>
							<CheckIcon
								class="absolute inset-0 size-4 text-accent-brand transition-all duration-150 ease-out {copied ? 'translate-y-0 opacity-100' : 'translate-y-2 opacity-0'}"
							/>
						</span>
					</Button>
				</div>
				<p class="text-xs text-muted-foreground">
					This link will only be shown once. Copy it now.
				</p>
			</div>
		{:else if existing && !showCreateForm}
			<div class="flex flex-col gap-4">
				<p class="text-sm text-muted-foreground">
					This {isDir ? "folder" : "file"} already has an active share link.
				</p>
				<div class="rounded-lg border border-border bg-background p-3 space-y-2 text-sm">
					<div class="flex justify-between">
						<span class="text-muted-foreground">Created</span>
						<span class="text-meta tabular-nums">{formatAbsoluteDateTime(existing.createdAt)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Expires</span>
						<span class="text-meta tabular-nums">{formatRemainingLong(existing.expiresAt)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Downloads</span>
						<span class="text-meta tabular-nums">{existing.downloadCount}</span>
					</div>
					{#if existing.hasPassword}
						<div class="flex justify-between">
							<span class="text-muted-foreground">Password</span>
							<span class="text-meta">Yes</span>
						</div>
					{/if}
				</div>
				<p class="text-xs text-muted-foreground">
					Lost the link? Replace it with a new one.
				</p>
			</div>
		{:else}
			<div class="flex flex-col gap-4">
				{#if existing}
					<p class="text-xs text-muted-foreground">
						The current link stops working when you create the new one.
					</p>
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
		{/if}
		</div>

		{#if shareUrl}
			<Dialog.Footer>
				<Button onclick={() => (open = false)} disabled={!closeEnabled}>Done</Button>
			</Dialog.Footer>
		{:else if existing && !showCreateForm}
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)}>Close</Button>
				<Button variant="destructive" onclick={() => (showCreateForm = true)}>
					Replace Link…
				</Button>
			</Dialog.Footer>
		{:else if !loading}
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)} disabled={submitting}>Cancel</Button>
				<Button
					variant={existing ? "destructive" : "default"}
					onclick={submit}
					loading={submitting}
					disabled={usePassword && !password}
				>
					{submitting ? "Creating…" : existing ? "Replace Link" : "Create Link"}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
