<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { listShares, deleteShare } from "$lib/api/shares.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { sharedPaths } from "$lib/stores/sharedPaths.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import LinkIcon from "@lucide/svelte/icons/link";
	import Link2OffIcon from "@lucide/svelte/icons/link-2-off";
	import XIcon from "@lucide/svelte/icons/x";
	import LockIcon from "@lucide/svelte/icons/lock";
	import FileIcon from "$lib/components/FileIcon.svelte";
	import type { ShareLink } from "$lib/types.js";
	import { changes } from "$lib/changes";
	import LoadingState from "$lib/components/LoadingState.svelte";
	import EmptyState from "$lib/components/EmptyState.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";
	import ListCard from "$lib/components/ListCard.svelte";
	import { basename, dirname } from "$lib/utils.js";
	import { formatRemainingShort } from "$lib/utils/format.js";

	let shares = $state<ShareLink[]>([]);
	let loading = $state(true);
	let deleteTarget = $state<ShareLink | null>(null);
	let deleteConfirmOpen = $state(false);
	let submitting = $state(false);

	async function load() {
		try {
			const res = await listShares();
			shares = res.shares;
		} catch {
			toast.error("Failed to load shares");
		} finally {
			loading = false;
		}
	}

	onMount(() => { load(); });

	$effect(() => {
		const offShare = changes.on("share.changed", load);
		const offBehind = changes.onBehind(load);
		return () => {
			offShare();
			offBehind();
		};
	});

	function confirmDelete(share: ShareLink) {
		deleteTarget = share;
		deleteConfirmOpen = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		submitting = true;
		try {
			await deleteShare(deleteTarget.id);
			sharedPaths.remove(deleteTarget.filePath);
			shares = shares.filter((s) => s.id !== deleteTarget!.id);
			toast.success("Share link revoked");
			deleteConfirmOpen = false;
			deleteTarget = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to delete share");
		} finally {
			submitting = false;
		}
	}

	function isExpired(share: ShareLink): boolean {
		return !!share.expiresAt && share.expiresAt <= Date.now() / 1000;
	}
</script>

<div class="flex flex-col gap-4 p-4">
	<!-- Header -->
	<PageHeader
		icon={LinkIcon}
		title="Shares"
		meta={shares.length > 0 ? `${shares.length} ${shares.length === 1 ? "link" : "links"}` : undefined}
	/>

	<!-- Content -->
	{#if !sharesEnabled.enabled}
		<EmptyState
			icon={Link2OffIcon}
			title="Sharing is disabled"
			description="Enable it in Settings to create share links."
		>
			<Button href="/settings" variant="outline" size="sm">Open Settings</Button>
		</EmptyState>
	{:else if loading}
		<LoadingState />
	{:else}
		<ListCard class="flex-col" gridCols="md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_60px]">
			{#snippet header()}
				<div>File</div>
				<div class="text-center">Access</div>
				<div class="text-center">Expires</div>
				<div class="text-center"></div>
			{/snippet}
			{#snippet children()}
				{#if shares.length === 0}
					<div class="px-[14px] py-12 text-center text-meta text-muted-foreground">
						No active shares. Create one from a file's context menu.
					</div>
				{:else}
					<div class="flex flex-col">
						{#each shares as share (share.id)}
						<div
							class="grid items-center border-b border-border transition-colors last:border-b-0 grid-cols-[1fr_auto] md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_60px] md:gap-3 px-[14px] py-3.5 md:py-[11px]"
						>
							<div class="flex min-w-0 items-center gap-3">
								<FileIcon
									isDir={share.isDir}
									name={basename(share.filePath)}
									class="size-6 shrink-0 {share.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
									strokeWidth={1.4}
								/>
								<div class="min-w-0 flex-1">
									<p class="flex items-center gap-1.5 truncate text-[15px] font-medium md:text-base">
										<span class="truncate">{basename(share.filePath)}</span>
										{#if share.hasPassword}
											<LockIcon class="size-3 shrink-0 text-muted-foreground md:hidden" strokeWidth={2.5} />
										{/if}
									</p>
									<p class="truncate font-mono text-meta text-muted-foreground">
										{dirname(share.filePath)}<span class="md:hidden"> · {#if isExpired(share)}<span class="text-destructive">Expired</span>{:else}{formatRemainingShort(share.expiresAt)}{/if}</span>
									</p>
								</div>
							</div>
							<div class="hidden text-center md:block">
								{#if share.hasPassword}
									<span class="inline-flex items-center gap-1 rounded-md border border-border-2 bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
										<LockIcon class="size-2.5" strokeWidth={2.5} />
										password
									</span>
								{:else}
									<span class="inline-flex items-center rounded-md border border-border-2 px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
										public
									</span>
								{/if}
							</div>
							<div class="flex shrink-0 items-center justify-end md:hidden">
								<Button
									variant="ghost"
									size="icon"
									class="cursor-pointer"
									onclick={() => confirmDelete(share)}
									title="Revoke link"
									aria-label="Revoke share link"
								>
									<XIcon class="size-4" strokeWidth={2} />
								</Button>
							</div>
							<div class="hidden text-center md:block">
								{#if isExpired(share)}
									<span class="inline-flex items-center rounded-md border border-destructive/40 bg-destructive/10 px-1.5 py-0.5 text-[11px] font-medium text-destructive">
										Expired
									</span>
								{:else}
									<span class="text-meta tabular-nums text-muted-foreground">
										{formatRemainingShort(share.expiresAt)}
									</span>
								{/if}
							</div>
							<div class="hidden text-center md:block">
								<Button
									variant="ghost"
									size="icon-xs"
									class="cursor-pointer"
									onclick={() => confirmDelete(share)}
									title="Revoke link"
									aria-label="Revoke share link"
								>
									<XIcon class="size-3.5" strokeWidth={2} />
								</Button>
							</div>
						</div>
						{/each}
					</div>
				{/if}
			{/snippet}
		</ListCard>
	{/if}
</div>

<!-- Delete Confirmation -->
<AlertDialog.Root bind:open={deleteConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke share link?</AlertDialog.Title>
			<AlertDialog.Description>
				Anyone with this link will lose access to "{deleteTarget?.filePath}". The file itself is not affected.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={handleDelete} disabled={submitting}>
				{submitting ? "Revoking…" : "Revoke"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
