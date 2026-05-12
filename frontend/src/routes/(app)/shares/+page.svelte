<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { listShares, deleteShare } from "$lib/api/shares.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { formatDate } from "$lib/utils/format.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Link, Link2Off, Trash2, FolderOpen, FileText, Lock } from "lucide-svelte";
	import type { ShareLink } from "$lib/types.js";

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

	function confirmDelete(share: ShareLink) {
		deleteTarget = share;
		deleteConfirmOpen = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		submitting = true;
		try {
			await deleteShare(deleteTarget.id);
			shares = shares.filter((s) => s.id !== deleteTarget!.id);
			toast.success("Share link deleted");
			deleteConfirmOpen = false;
			deleteTarget = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to delete share");
		} finally {
			submitting = false;
		}
	}

	function formatExpiry(share: ShareLink): string {
		if (!share.expiresAt) return "Never";
		const now = Date.now() / 1000;
		const remaining = share.expiresAt - now;
		if (remaining <= 0) return "Expired";
		if (remaining < 3600) return `${Math.ceil(remaining / 60)}m`;
		if (remaining < 86400) return `${Math.ceil(remaining / 3600)}h`;
		return `${Math.ceil(remaining / 86400)}d`;
	}

	function fileName(path: string): string {
		return path.split("/").pop() ?? path;
	}

	function parentDir(path: string): string {
		const i = path.lastIndexOf("/");
		return i <= 0 ? "/" : path.substring(0, i);
	}
</script>

<div class="flex h-full flex-col gap-4 p-4">
	<!-- Header -->
	<div class="flex items-center gap-2">
		<Link class="size-5 text-muted-foreground" />
		<h1 class="text-lg font-semibold">Shares</h1>
		{#if shares.length > 0}
			<span class="text-sm text-muted-foreground">({shares.length} {shares.length === 1 ? "link" : "links"})</span>
		{/if}
	</div>

	<!-- Content -->
	{#if !sharesEnabled.enabled}
		<div class="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
			<Link2Off class="size-12 opacity-30" />
			<p class="text-sm">Sharing is disabled</p>
			<p class="text-xs">Enable it in Settings to create share links.</p>
		</div>
	{:else if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading...
		</div>
	{:else if shares.length === 0}
		<div class="flex flex-col items-center justify-center gap-2 py-24 text-muted-foreground">
			<Link class="size-12 opacity-30" />
			<p>No active share links</p>
		</div>
	{:else}
		<!-- Table header -->
		<div class="flex border-b border-border text-xs text-muted-foreground">
			<div class="flex-1 py-2 pl-4 font-medium">File</div>
			<div class="w-28 py-2 font-medium">Expires</div>
			<div class="w-24 py-2 text-right font-medium">Downloads</div>
			<div class="w-20 py-2 pr-4 text-right font-medium">Actions</div>
		</div>

		<!-- Rows -->
		<div class="flex flex-col overflow-auto">
			{#each shares as share (share.id)}
				<div class="group flex items-center border-b border-border/50 transition-colors hover:bg-accent/50">
					<div class="flex flex-1 items-center gap-2 py-2.5 pl-4 text-sm">
						{#if share.isDir}
							<FolderOpen class="size-4 shrink-0 text-muted-foreground" />
						{:else}
							<FileText class="size-4 shrink-0 text-muted-foreground" />
						{/if}
						<div class="min-w-0">
							<p class="truncate font-medium">{fileName(share.filePath)}</p>
							<p class="truncate text-xs text-muted-foreground">{parentDir(share.filePath)}</p>
						</div>
						{#if share.hasPassword}
							<Lock class="size-3 shrink-0 text-muted-foreground" />
						{/if}
					</div>
					<div class="w-28 py-2.5 text-sm text-muted-foreground">
						{formatExpiry(share)}
					</div>
					<div class="w-24 py-2.5 text-right text-sm text-muted-foreground">
						{share.downloadCount}
					</div>
					<div class="w-20 py-2.5 pr-4 text-right">
						<Button
							variant="ghost"
							size="icon-xs"
							class="text-muted-foreground hover:text-destructive"
							onclick={() => confirmDelete(share)}
						>
							<Trash2 class="size-3.5" />
						</Button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Delete Confirmation -->
<AlertDialog.Root bind:open={deleteConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete share link?</AlertDialog.Title>
			<AlertDialog.Description>
				This will revoke the share link for "{deleteTarget?.filePath}". Anyone with the link will no longer be able to access the file.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={handleDelete} disabled={submitting}>
				{submitting ? "Deleting..." : "Delete"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
