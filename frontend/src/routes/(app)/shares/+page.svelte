<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { listShares, deleteShare } from "$lib/api/shares.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { sharedPaths } from "$lib/stores/sharedPaths.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Link, Link2Off, X, Lock } from "lucide-svelte";
	import FileIcon from "$lib/components/FileIcon.svelte";
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
			sharedPaths.remove(deleteTarget.filePath);
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

<div class="flex flex-col gap-4 p-4">
	<!-- Header -->
	<div class="flex items-center gap-2">
		<Link class="size-5 text-muted-foreground" strokeWidth={2} />
		<h1 class="text-lg font-bold tracking-[-0.01em]">Shares</h1>
		{#if shares.length > 0}
			<span class="text-[13px] tabular-nums text-muted-foreground">
				{shares.length} {shares.length === 1 ? "link" : "links"}
			</span>
		{/if}
	</div>

	<!-- Content -->
	{#if !sharesEnabled.enabled}
		<div class="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
			<Link2Off class="size-12 opacity-30" strokeWidth={1.5} />
			<p class="text-[15px]">Sharing is disabled</p>
			<p class="text-[13px]">Enable it in Settings to create share links.</p>
		</div>
	{:else if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading…
		</div>
	{:else}
		<div class="flex flex-col overflow-hidden rounded-xl border border-border bg-card">
			<!-- Table header (desktop) -->
			<div
				class="hidden border-b border-border bg-[oklch(0_0_0/0.2)] text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_60px] md:gap-3 md:px-[14px] md:py-2.5"
			>
				<div>File</div>
				<div class="text-center">Access</div>
				<div class="text-center">Expires</div>
				<div class="text-center"></div>
			</div>

			{#if shares.length === 0}
				<div class="px-[14px] py-12 text-center text-[13px] text-muted-foreground">
					No active shares. Create one from a file's context menu.
				</div>
			{:else}
				<!-- Rows -->
				<div class="flex flex-col">
					{#each shares as share (share.id)}
					<div
						class="grid items-center border-b border-border transition-colors last:border-b-0 grid-cols-[1fr_auto] md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_60px] md:gap-3 px-[14px] py-3.5 md:py-[11px]"
					>
						<div class="flex min-w-0 items-center gap-3">
							<FileIcon
								isDir={share.isDir}
								name={fileName(share.filePath)}
								class="size-6 shrink-0 {share.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
								strokeWidth={1.4}
							/>
							<div class="min-w-0 flex-1">
								<p class="truncate text-[15px] font-medium md:text-base">
									{fileName(share.filePath)}
								</p>
								<p class="truncate font-mono text-[13px] text-muted-foreground">
									{parentDir(share.filePath)}
								</p>
							</div>
						</div>
						<div class="hidden text-center md:block">
							{#if share.hasPassword}
								<span class="inline-flex items-center gap-1 rounded-md border border-border-2 bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
									<Lock class="size-2.5" strokeWidth={2.5} />
									password
								</span>
							{:else}
								<span class="inline-flex items-center rounded-md border border-border-2 px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
									public
								</span>
							{/if}
						</div>
						<div class="flex shrink-0 items-center justify-end text-[13px] tabular-nums text-muted-foreground md:hidden">
							<Button
								variant="ghost"
								size="icon-xs"
								class="cursor-pointer"
								onclick={() => confirmDelete(share)}
								title="Revoke link"
							>
								<X class="size-3.5" strokeWidth={2} />
							</Button>
						</div>
						<div class="hidden text-center text-[13px] tabular-nums text-muted-foreground md:block">
							{formatExpiry(share)}
						</div>
						<div class="hidden text-center md:block">
							<Button
								variant="ghost"
								size="icon-xs"
								class="cursor-pointer"
								onclick={() => confirmDelete(share)}
								title="Revoke link"
							>
								<X class="size-3.5" strokeWidth={2} />
							</Button>
						</div>
					</div>
					{/each}
				</div>
			{/if}
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
