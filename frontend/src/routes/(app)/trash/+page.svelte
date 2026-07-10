<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { listTrash, restoreTrashItem, permanentDeleteTrashItem, emptyTrash } from "$lib/api/trash.js";
	import { getSettings } from "$lib/api/settings.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import type { TrashItem } from "$lib/types";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as ContextMenu from "$lib/components/ui/context-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import FileIcon from "$lib/components/FileIcon.svelte";
	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { trashEnabled } from "$lib/stores/trashEnabled.svelte.js";
	import { Trash2, RotateCcw, Info } from "lucide-svelte";
	import { changes } from "$lib/changes";

	let items = $state<TrashItem[]>([]);
	let loading = $state(true);
	let emptyConfirmOpen = $state(false);
	let deleteConfirmOpen = $state(false);
	let bulkDeleteConfirmOpen = $state(false);
	let deleteTarget = $state<TrashItem | null>(null);
	let submitting = $state(false);

	// Selection state
	let selected = $state<Set<string>>(new Set());
	let lastSelected = $state<string | null>(null);

	// Auto-purge config — drives header subtitle and per-row "purges in" cell.
	// `null` = settings fetch failed; hide subtitle/cell.
	let purgeAgeHours = $state<number | null>(null);

	const allIds = $derived(items.map((i) => i.id));

	// Parse a Go duration string ("720h", "30m", "0", "1h30m") into hours.
	// Bare numbers are treated as seconds (Go convention).
	function parseDurationHours(s: string): number {
		if (!s) return 0;
		let totalSec = 0;
		let any = false;
		const re = /(\d+(?:\.\d+)?)(h|m|s)/g;
		let m: RegExpExecArray | null;
		while ((m = re.exec(s)) !== null) {
			any = true;
			const n = parseFloat(m[1]);
			if (m[2] === "h") totalSec += n * 3600;
			else if (m[2] === "m") totalSec += n * 60;
			else totalSec += n;
		}
		if (!any) {
			const n = parseFloat(s);
			if (!isNaN(n)) return n / 3600;
		}
		return totalSec / 3600;
	}

	const purgeSubtitle = $derived.by(() => {
		if (purgeAgeHours === null) return null;
		if (purgeAgeHours <= 0) return "Items never auto-purge.";
		if (purgeAgeHours >= 24) {
			const days = Math.floor(purgeAgeHours / 24);
			return `Items auto-purge after ${days} ${days === 1 ? "day" : "days"}.`;
		}
		const h = Math.floor(purgeAgeHours);
		return `Items auto-purge after ${h} ${h === 1 ? "hour" : "hours"}.`;
	});

	function purgesIn(deletedAt: number): string {
		if (purgeAgeHours === null || purgeAgeHours <= 0) return "—";
		const purgeAtMs = deletedAt * 1000 + purgeAgeHours * 3600 * 1000;
		const remainingMs = purgeAtMs - Date.now();
		if (remainingMs <= 0) return "purges soon";
		const days = Math.ceil(remainingMs / 86400000);
		if (days >= 1) return `purges in ${days}d`;
		const hours = Math.ceil(remainingMs / 3600000);
		return `purges in ${hours}h`;
	}

	function handleItemClick(e: MouseEvent, item: TrashItem) {
		e.stopPropagation();
		if (e.shiftKey && lastSelected) {
			e.preventDefault();
			const start = allIds.indexOf(lastSelected);
			const end = allIds.indexOf(item.id);
			if (start !== -1 && end !== -1) {
				const lo = Math.min(start, end);
				const hi = Math.max(start, end);
				const next = new Set(selected);
				for (let i = lo; i <= hi; i++) next.add(allIds[i]);
				selected = next;
				lastSelected = item.id;
			}
		} else if (e.ctrlKey || e.metaKey) {
			e.preventDefault();
			const next = new Set(selected);
			if (next.has(item.id)) next.delete(item.id);
			else next.add(item.id);
			selected = next;
			lastSelected = item.id;
		} else {
			if (selected.size === 1 && selected.has(item.id)) {
				selected = new Set();
				lastSelected = null;
			} else {
				selected = new Set([item.id]);
				lastSelected = item.id;
			}
		}
	}

	function clearSelection() {
		selected = new Set();
		lastSelected = null;
	}

	function getContextIds(item: TrashItem): string[] {
		return selected.has(item.id) && selected.size > 1 ? [...selected] : [item.id];
	}

	function itemName(item: TrashItem): string {
		return item.originalPath.split("/").pop() ?? "";
	}

	function parentDir(path: string): string {
		const i = path.lastIndexOf("/");
		return i <= 0 ? "/" : path.substring(0, i);
	}

	async function load() {
		try {
			const [trashRes, settings] = await Promise.all([
				listTrash(),
				getSettings().catch(() => null),
			]);
			items = trashRes.items;
			trashCount.set(items.length);
			if (settings) {
				purgeAgeHours = parseDurationHours(settings.values["trash.purge_age"] ?? "720h");
			}
		} catch {
			toast.error("Failed to load trash");
		} finally {
			loading = false;
		}
	}

	onMount(() => { load(); });

	$effect(() => {
		const offTrash = changes.on("trash.changed", load);
		const offBehind = changes.onBehind(load);
		return () => {
			offTrash();
			offBehind();
		};
	});

	async function handleRestore(item: TrashItem) {
		try {
			await restoreTrashItem(item.id);
			toast.success(`Restored "${itemName(item)}"`);
			items = items.filter((i) => i.id !== item.id);
			selected.delete(item.id);
			selected = new Set(selected);
			if (lastSelected === item.id) lastSelected = null;
			trashCount.set(items.length);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Restore failed");
		}
	}

	async function handleBulkRestore() {
		const ids = [...selected];
		let restored = 0;
		let failed = 0;
		for (const id of ids) {
			try {
				await restoreTrashItem(id);
				restored++;
			} catch {
				failed++;
			}
		}
		if (failed === 0) {
			items = items.filter((i) => !ids.includes(i.id));
			toast.success(`Restored ${restored} item${restored !== 1 ? "s" : ""}`);
		} else {
			await load();
			toast.error(`${failed} item(s) failed to restore`);
		}
		trashCount.set(items.length);
		clearSelection();
	}

	function confirmDelete(item: TrashItem) {
		deleteTarget = item;
		deleteConfirmOpen = true;
	}

	function confirmBulkDelete() {
		bulkDeleteConfirmOpen = true;
	}

	async function handlePermanentDelete() {
		if (!deleteTarget) return;
		submitting = true;
		try {
			await permanentDeleteTrashItem(deleteTarget.id);
			toast.success(`Permanently deleted "${itemName(deleteTarget)}"`);
			items = items.filter((i) => i.id !== deleteTarget!.id);
			selected.delete(deleteTarget!.id);
			selected = new Set(selected);
			if (lastSelected === deleteTarget!.id) lastSelected = null;
			trashCount.set(items.length);
			deleteConfirmOpen = false;
			deleteTarget = null;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Delete failed");
		} finally {
			submitting = false;
		}
	}

	async function handleBulkPermanentDelete() {
		const ids = [...selected];
		submitting = true;
		let deleted = 0;
		let failed = 0;
		for (const id of ids) {
			try {
				await permanentDeleteTrashItem(id);
				deleted++;
			} catch {
				failed++;
			}
		}
		if (failed === 0) {
			items = items.filter((i) => !ids.includes(i.id));
			toast.success(`Permanently deleted ${deleted} item${deleted !== 1 ? "s" : ""}`);
		} else {
			await load();
			toast.error(`${failed} item(s) failed to delete`);
		}
		trashCount.set(items.length);
		clearSelection();
		bulkDeleteConfirmOpen = false;
		submitting = false;
	}

	async function handleEmptyTrash() {
		submitting = true;
		try {
			await emptyTrash();
			toast.success("Trash emptied");
			items = [];
			trashCount.set(0);
			clearSelection();
			emptyConfirmOpen = false;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to empty trash");
		} finally {
			submitting = false;
		}
	}

	async function handleContextRestore(item: TrashItem) {
		const ids = getContextIds(item);
		if (ids.length === 1) {
			await handleRestore(item);
		} else {
			await handleBulkRestore();
		}
	}

	function handleContextDelete(item: TrashItem) {
		const ids = getContextIds(item);
		if (ids.length === 1) {
			confirmDelete(item);
		} else {
			confirmBulkDelete();
		}
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="flex min-h-full flex-col gap-4 p-4" onclick={clearSelection}>
	<!-- Header (always visible) -->
	<div class="flex items-center gap-3">
		<div class="flex flex-1 flex-col">
			<div class="flex items-center gap-2">
				<Trash2 class="size-5 text-muted-foreground" strokeWidth={2} />
				<h1 class="text-lg font-bold tracking-[-0.01em]">Trash</h1>
				{#if items.length > 0}
					<span class="text-[13px] tabular-nums text-muted-foreground">
						{items.length} {items.length === 1 ? "item" : "items"}
					</span>
				{/if}
			</div>
			{#if purgeSubtitle}
				<p class="mt-1 text-[13px] text-muted-foreground">{purgeSubtitle}</p>
			{/if}
		</div>

		{#if items.length > 0}
			<Button
				variant="destructive"
				size="default"
				onclick={(e) => { e.stopPropagation(); emptyConfirmOpen = true; }}
			>
				<Trash2 class="size-4" strokeWidth={2} />
				<span>Empty trash</span>
			</Button>
		{/if}
	</div>

	<!-- Selection toolbar (only when active) -->
	{#if selected.size > 0}
		<div class="flex items-center gap-2">
			<span
				class="inline-flex min-w-[5.5rem] items-center justify-center rounded-md bg-accent-brand-dim px-2.5 py-1 text-[13px] font-medium tabular-nums text-accent-brand"
			>
				{selected.size} selected
			</span>
			<Button
				variant="outline"
				size="sm"
				onclick={(e) => { e.stopPropagation(); handleBulkRestore(); }}
			>
				<RotateCcw class="size-[15px]" strokeWidth={2} />
				<span>Restore</span>
			</Button>
			<Button
				variant="outline"
				size="sm"
				class="text-destructive hover:bg-destructive/10 hover:text-destructive"
				onclick={(e) => { e.stopPropagation(); confirmBulkDelete(); }}
			>
				<Trash2 class="size-[15px]" strokeWidth={2} />
				<span>Delete permanently</span>
			</Button>
		</div>
	{/if}

	<!-- Disabled banner (items still present after disable) -->
	{#if !trashEnabled.enabled && items.length > 0 && !loading}
		<div class="flex items-start gap-2.5 rounded-lg border border-border bg-muted/40 px-3.5 py-2.5 text-[13px] text-muted-foreground">
			<Info class="mt-px size-4 shrink-0" strokeWidth={2} />
			<span>
				Trash is disabled — new deletions are permanent. Existing items can still be restored or purged.
			</span>
		</div>
	{/if}

	<!-- Content -->
	{#if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading…
		</div>
	{:else if !trashEnabled.enabled && items.length === 0}
		<div class="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
			<Trash2 class="size-12 opacity-30" strokeWidth={1.5} />
			<p class="text-[15px]">Trash is disabled</p>
			<p class="text-[13px]">Enable it in Settings to keep deleted files recoverable.</p>
		</div>
	{:else if items.length === 0}
		<div class="rounded-xl border border-border bg-card p-12 text-center">
			<Trash2 class="mx-auto size-8 text-muted-foreground" strokeWidth={1.5} />
			<div class="mt-3 text-sm font-medium">Trash is empty</div>
			<div class="mt-1 text-[13px] text-muted-foreground">Deleted items will appear here.</div>
		</div>
	{:else}
		<!-- List -->
		<div class="flex flex-col overflow-hidden rounded-xl border border-border bg-card">
			<!-- Table header (desktop) -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="hidden border-b border-border bg-[oklch(0_0_0/0.2)] text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_72px] md:gap-3 md:px-[14px] md:py-2.5"
				onclick={(e) => e.stopPropagation()}
			>
				<div>Name</div>
				<div class="text-center">Deleted</div>
				<div class="text-center">Size</div>
				<div class="text-center">Purges in</div>
				<div></div>
			</div>

			<!-- Rows -->
			<div class="flex flex-col">
				{#each items as item (item.id)}
					{@const isSelected = selected.has(item.id)}
					{@const parent = parentDir(item.originalPath)}
					<ContextMenu.Root>
						<ContextMenu.Trigger>
							{#snippet child({ props })}
								<div
									{...props}
									class="grid cursor-pointer items-center border-b border-border transition-colors select-none last:border-b-0 grid-cols-[1fr_auto] md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_72px] md:gap-3 px-[14px] py-2.5 md:py-2
										{isSelected ? 'bg-accent-brand-dim' : ''}"
									onclick={(e) => handleItemClick(e, item)}
									role="row"
									tabindex={0}
								>
									<div class="flex min-w-0 items-center gap-3">
										<FileIcon
											isDir={item.isDir}
											name={itemName(item)}
											class="size-8 shrink-0 {item.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
											strokeWidth={1.4}
										/>
										<div class="flex min-w-0 flex-1 flex-col">
											<span class="truncate text-[15px] font-medium md:text-[15px]">
												{itemName(item)}
											</span>
											<span
												class="truncate font-mono text-[11px] text-muted-foreground"
												title={item.originalPath}
											>
												{parent}
											</span>
										</div>
									</div>
									<div class="flex shrink-0 items-center text-xs tabular-nums text-muted-foreground md:hidden">
										{item.isDir ? "—" : formatFileSize(item.size)}
									</div>
									<div class="hidden text-center text-[13px] tabular-nums text-muted-foreground md:block">
										{formatDate(item.deletedAt)}
									</div>
									<div class="hidden text-center text-[13px] tabular-nums text-muted-foreground md:block">
										{item.isDir ? "—" : formatFileSize(item.size)}
									</div>
									<div class="hidden text-center text-[13px] tabular-nums text-muted-foreground md:block">
										{purgesIn(item.deletedAt)}
									</div>
									<!-- svelte-ignore a11y_no_static_element_interactions -->
									<div
										class="hidden justify-end md:flex"
										onclick={(e) => e.stopPropagation()}
									>
										<Button
											variant="outline"
											size="xs"
											onclick={() => handleRestore(item)}
										>
											Restore
										</Button>
									</div>
								</div>
							{/snippet}
						</ContextMenu.Trigger>
						<ContextMenu.Content class="w-48">
							<ContextMenu.Item onclick={() => handleContextRestore(item)}>
								{selected.has(item.id) && selected.size > 1 ? `Restore ${selected.size} items` : "Restore"}
							</ContextMenu.Item>
							<ContextMenu.Separator />
							<ContextMenu.Item variant="destructive" onclick={() => handleContextDelete(item)}>
								{selected.has(item.id) && selected.size > 1 ? `Delete ${selected.size} items` : "Delete permanently"}
							</ContextMenu.Item>
						</ContextMenu.Content>
					</ContextMenu.Root>
				{/each}
			</div>
		</div>
	{/if}
</div>

<!-- Empty Trash Confirmation -->
<AlertDialog.Root bind:open={emptyConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Empty trash?</AlertDialog.Title>
			<AlertDialog.Description>
				Permanently delete all {items.length} {items.length === 1 ? "item" : "items"}? This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={handleEmptyTrash} disabled={submitting}>
				{submitting ? "Deleting..." : "Empty Trash"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<!-- Single Item Delete Confirmation -->
<AlertDialog.Root bind:open={deleteConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Permanently delete "{deleteTarget?.originalPath.split("/").pop()}"?</AlertDialog.Title>
			<AlertDialog.Description>This action cannot be undone.</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={handlePermanentDelete} disabled={submitting}>
				{submitting ? "Deleting..." : "Delete"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<!-- Bulk Delete Confirmation -->
<AlertDialog.Root bind:open={bulkDeleteConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Permanently delete {selected.size} {selected.size === 1 ? "item" : "items"}?</AlertDialog.Title>
			<AlertDialog.Description>This action cannot be undone.</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={handleBulkPermanentDelete} disabled={submitting}>
				{submitting ? "Deleting..." : "Delete"}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
