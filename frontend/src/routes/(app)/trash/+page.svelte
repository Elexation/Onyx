<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import {
		listTrash,
		restoreTrashItem,
		permanentDeleteTrashItem,
		emptyTrash,
		checkRestoreConflicts,
		type RestoreStrategy,
		type RestoreConflictItem,
	} from "$lib/api/trash.js";
	import { getSettings } from "$lib/api/settings.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import { basename, dirname } from "$lib/utils.js";
	import type { TrashItem } from "$lib/types";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as ContextMenu from "$lib/components/ui/context-menu/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import FileIcon from "$lib/components/FileIcon.svelte";
	import ConflictDialog, { type ConflictPair } from "$lib/components/dialogs/ConflictDialog.svelte";
	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { trashEnabled } from "$lib/stores/trashEnabled.svelte.js";
	import { viewport } from "$lib/stores/viewport.svelte.js";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import InfoIcon from "@lucide/svelte/icons/info";
	import { changes } from "$lib/changes";
	import LoadingState from "$lib/components/LoadingState.svelte";
	import EmptyState from "$lib/components/EmptyState.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";

	let items = $state<TrashItem[]>([]);
	let loading = $state(true);
	let emptyConfirmOpen = $state(false);
	let deleteConfirmOpen = $state(false);
	let bulkDeleteConfirmOpen = $state(false);
	let deleteTarget = $state<TrashItem | null>(null);
	let submitting = $state(false);

	// Snapshot counts at dialog-open time so the title doesn't flicker when
	// items[]/selected mutate during the await (or the change feed lands
	// mid-flight).
	let emptyConfirmCount = $state(0);
	let bulkDeleteConfirmCount = $state(0);

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

	// Coalesce trash.changed bursts (e.g. parallel bulk-delete emits N events
	// in quick succession) into one trailing-edge listTrash() refetch.
	$effect(() => {
		let debounceTimer: ReturnType<typeof setTimeout> | null = null;
		const debouncedLoad = () => {
			if (debounceTimer) clearTimeout(debounceTimer);
			debounceTimer = setTimeout(load, 250);
		};
		const offTrash = changes.on("trash.changed", debouncedLoad);
		const offBehind = changes.onBehind(load);
		return () => {
			if (debounceTimer) clearTimeout(debounceTimer);
			offTrash();
			offBehind();
		};
	});

	// Conflict-resolution dialog state. Resolver promise pattern lets the
	// async restore loop await the user's choice without callback gymnastics.
	let restoreConflictOpen = $state(false);
	let restoreConflictPairs = $state<ConflictPair[]>([]);
	let restoreResolver: ((r: Record<string, RestoreStrategy>) => void) | null = null;

	function awaitConflictResolution(pairs: ConflictPair[]) {
		return new Promise<Record<string, RestoreStrategy>>((resolve) => {
			restoreConflictPairs = pairs;
			restoreResolver = resolve;
			restoreConflictOpen = true;
		});
	}

	function handleRestoreConflictResolve(resolutions: Record<string, RestoreStrategy>) {
		restoreConflictOpen = false;
		const r = restoreResolver;
		restoreResolver = null;
		if (r) r(resolutions);
	}

	type RestoreTally = "plain" | "replaced" | "keptBoth" | "skipped" | "failed";

	// tryRestore wraps a single restore call with mid-batch 409 recovery.
	// When two trash items target the same OriginalPath, the upfront
	// pre-check sees the path empty (0 conflicts), the first restore wins,
	// and the second collides at the server. We fetch the now-real conflict
	// and surface the dialog inline so the user can pick a per-item strategy.
	async function tryRestore(
		id: string,
		strategy: RestoreStrategy | undefined,
	): Promise<RestoreTally> {
		try {
			await restoreTrashItem(id, strategy);
			if (strategy === "replace") return "replaced";
			if (strategy === "keepBoth") return "keptBoth";
			return "plain";
		} catch (e) {
			const status = (e as { status?: number })?.status;
			if (status !== 409) return "failed";

			// Server now sees the just-restored peer as the conflicting "existing".
			let conflictPair: ConflictPair | null = null;
			try {
				const { conflicts } = await checkRestoreConflicts([id]);
				if (conflicts.length > 0) {
					const c = conflicts[0];
					conflictPair = { path: c.path, existing: c.existing, incoming: c.restoring };
				}
			} catch {
				return "failed";
			}
			if (!conflictPair) return "failed";

			const resolutions = await awaitConflictResolution([conflictPair]);
			const chosen = resolutions[conflictPair.path];
			if (!chosen || chosen === "skip") return "skipped";

			try {
				await restoreTrashItem(id, chosen);
				if (chosen === "replace") return "replaced";
				if (chosen === "keepBoth") return "keptBoth";
				return "plain";
			} catch {
				return "failed";
			}
		}
	}

	// Unified restore flow: single-row Restore, bulk-toolbar Restore, and
	// context-menu Restore all funnel through here. Pre-checks for conflicts,
	// prompts via ConflictDialog if any, then restores each item with the
	// chosen strategy. Selection mutations are scoped to actually-restored ids.
	async function restoreItems(ids: string[]) {
		if (ids.length === 0) return;

		let conflicts: RestoreConflictItem[] = [];
		try {
			({ conflicts } = await checkRestoreConflicts(ids));
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to check for conflicts");
			return;
		}

		let resolutions: Record<string, RestoreStrategy> = {};
		if (conflicts.length > 0) {
			const pairs: ConflictPair[] = conflicts.map((c) => ({
				path: c.path,
				existing: c.existing,
				incoming: c.restoring,
			}));
			resolutions = await awaitConflictResolution(pairs);
		}

		const conflictPathByID = new Map(conflicts.map((c) => [c.id, c.path]));

		// Capture single-item name before mutating items.
		const singleItemName =
			ids.length === 1
				? basename(items.find((i) => i.id === ids[0])?.originalPath ?? "")
				: "";

		let plain = 0;
		let replaced = 0;
		let keptBoth = 0;
		let skipped = 0;
		let failed = 0;
		const restoredIds: string[] = [];

		for (const id of ids) {
			const conflictPath = conflictPathByID.get(id);
			const strategy = conflictPath ? resolutions[conflictPath] : undefined;
			if (conflictPath && (!strategy || strategy === "skip")) {
				skipped++;
				continue;
			}
			const tally = await tryRestore(id, strategy);
			if (tally === "failed") failed++;
			else if (tally === "skipped") skipped++;
			else {
				restoredIds.push(id);
				if (tally === "replaced") replaced++;
				else if (tally === "keptBoth") keptBoth++;
				else plain++;
			}
		}

		if (failed > 0) {
			await load();
		} else {
			const restoredSet = new Set(restoredIds);
			items = items.filter((i) => !restoredSet.has(i.id));
		}
		for (const id of restoredIds) selected.delete(id);
		selected = new Set(selected);
		if (lastSelected !== null && restoredIds.includes(lastSelected)) lastSelected = null;
		trashCount.set(items.length);

		const totalRestored = plain + replaced + keptBoth;
		if (failed > 0) {
			toast.error(`${failed} item${failed !== 1 ? "s" : ""} failed to restore`);
		} else if (totalRestored === 0 && skipped > 0) {
			toast.info(`Skipped ${skipped} item${skipped !== 1 ? "s" : ""}`);
		} else if (ids.length === 1 && totalRestored === 1 && skipped === 0) {
			toast.success(`Restored "${singleItemName}"`);
		} else if (totalRestored > 0) {
			const parts: string[] = [];
			if (replaced) parts.push(`${replaced} replaced`);
			if (keptBoth) parts.push(`${keptBoth} kept both`);
			if (skipped) parts.push(`${skipped} skipped`);
			const suffix = parts.length ? ` (${parts.join(" · ")})` : "";
			toast.success(`Restored ${totalRestored} item${totalRestored !== 1 ? "s" : ""}${suffix}`);
		}
	}

	function confirmDelete(item: TrashItem) {
		deleteTarget = item;
		deleteConfirmOpen = true;
	}

	function confirmBulkDelete() {
		bulkDeleteConfirmCount = selected.size;
		bulkDeleteConfirmOpen = true;
	}

	async function handlePermanentDelete() {
		if (!deleteTarget) return;
		submitting = true;
		try {
			await permanentDeleteTrashItem(deleteTarget.id);
			toast.success(`Permanently deleted "${basename(deleteTarget.originalPath)}"`);
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
		const results = await Promise.allSettled(
			ids.map((id) => permanentDeleteTrashItem(id)),
		);
		const failed = results.filter((r) => r.status === "rejected").length;
		const deleted = results.length - failed;
		if (failed === 0) {
			const idSet = new Set(ids);
			items = items.filter((i) => !idSet.has(i.id));
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
		await restoreItems(getContextIds(item));
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
	<PageHeader
		icon={Trash2Icon}
		title="Trash"
		meta={items.length > 0 ? `${items.length} ${items.length === 1 ? "item" : "items"}` : undefined}
		subtitle={purgeSubtitle ?? undefined}
	>
		{#snippet action()}
			{#if items.length > 0}
				<Button
					variant="destructive"
					size="default"
					onclick={(e) => { e.stopPropagation(); emptyConfirmCount = items.length; emptyConfirmOpen = true; }}
				>
					<Trash2Icon class="size-4" strokeWidth={2} />
					<span>Empty trash</span>
				</Button>
			{/if}
		{/snippet}
	</PageHeader>

	<!-- Selection toolbar (only when active) -->
	{#if selected.size > 0}
		<div class="flex items-center gap-2">
			<span
				class="inline-flex min-w-[5.5rem] items-center justify-center rounded-md bg-accent-brand-dim px-2.5 py-1 text-meta font-medium tabular-nums text-accent-brand"
			>
				{selected.size} selected
			</span>
			<Button
				variant="outline"
				size="sm"
				onclick={(e) => { e.stopPropagation(); restoreItems([...selected]); }}
			>
				<RotateCcwIcon class="size-[15px]" strokeWidth={2} />
				<span>Restore</span>
			</Button>
			<Button
				variant="outline"
				size="sm"
				class="text-destructive hover:bg-destructive/10 hover:text-destructive"
				onclick={(e) => { e.stopPropagation(); confirmBulkDelete(); }}
			>
				<Trash2Icon class="size-[15px]" strokeWidth={2} />
				<span>Delete permanently</span>
			</Button>
		</div>
	{/if}

	<!-- Disabled banner (items still present after disable) -->
	{#if !trashEnabled.enabled && items.length > 0 && !loading}
		<div class="flex items-start gap-2.5 rounded-lg border border-border bg-muted/40 px-3.5 py-2.5 text-meta text-muted-foreground">
			<InfoIcon class="mt-px size-4 shrink-0" strokeWidth={2} />
			<span>
				Trash is disabled — new deletions are permanent. Existing items can still be restored or purged.
			</span>
		</div>
	{/if}

	<!-- Content -->
	{#if loading}
		<LoadingState />
	{:else if !trashEnabled.enabled && items.length === 0}
		<EmptyState
			icon={Trash2Icon}
			title="Trash is disabled"
			description="Enable it in Settings to keep deleted files recoverable."
		/>
	{:else if items.length === 0}
		<EmptyState
			icon={Trash2Icon}
			title="Trash is empty"
			description="Deleted items will appear here."
		/>
	{:else}
		<!-- List -->
		<div class="flex flex-col overflow-hidden rounded-xl border border-border bg-card">
			<!-- Table header (desktop) -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="hidden border-b border-border bg-list-header text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_72px] md:gap-3 md:px-[14px] md:py-2.5"
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
					{@const parent = dirname(item.originalPath)}
					<ContextMenu.Root>
						<ContextMenu.Trigger disabled={viewport.isMobile}>
							{#snippet child({ props })}
								<div
									{...props}
									class="grid cursor-pointer items-center border-b border-border transition-colors select-none last:border-b-0 grid-cols-[1fr_auto] md:grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_72px] md:gap-3 px-[14px] py-2.5 md:py-2
										{isSelected ? 'bg-accent-brand-dim' : ''}"
									onclick={(e) => handleItemClick(e, item)}
									oncontextmenucapture={(e) => {
										if (viewport.isMobile) {
											e.preventDefault();
											e.stopImmediatePropagation();
										}
									}}
									role="row"
									tabindex={0}
								>
									<div class="flex min-w-0 items-center gap-3">
										<FileIcon
											isDir={item.isDir}
											name={basename(item.originalPath)}
											class="size-8 shrink-0 {item.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
											strokeWidth={1.4}
										/>
										<div class="flex min-w-0 flex-1 flex-col">
											<span class="truncate text-[15px] font-medium md:text-[15px]">
												{basename(item.originalPath)}
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
									<div class="hidden text-center text-meta tabular-nums text-muted-foreground md:block">
										{formatDate(item.deletedAt)}
									</div>
									<div class="hidden text-center text-meta tabular-nums text-muted-foreground md:block">
										{item.isDir ? "—" : formatFileSize(item.size)}
									</div>
									<div class="hidden text-center text-meta tabular-nums text-muted-foreground md:block">
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
											onclick={() => restoreItems([item.id])}
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
				Permanently delete all {emptyConfirmCount} {emptyConfirmCount === 1 ? "item" : "items"}? This action cannot be undone.
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
			<AlertDialog.Title>Permanently delete "{deleteTarget ? basename(deleteTarget.originalPath) : ''}"?</AlertDialog.Title>
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
			<AlertDialog.Title>Permanently delete {bulkDeleteConfirmCount} {bulkDeleteConfirmCount === 1 ? "item" : "items"}?</AlertDialog.Title>
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

<!-- Restore conflict resolution -->
{#if restoreConflictOpen}
	<ConflictDialog
		conflicts={restoreConflictPairs}
		kind="restore"
		onresolve={handleRestoreConflictResolve}
	/>
{/if}
