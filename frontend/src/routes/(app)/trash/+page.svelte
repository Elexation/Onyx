<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { listTrash, restoreTrashItem, permanentDeleteTrashItem, emptyTrash } from "$lib/api/trash.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import type { TrashItem } from "$lib/types";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import * as ContextMenu from "$lib/components/ui/context-menu/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import FileIcon from "$lib/components/FileIcon.svelte";
	import VirtualList from "$lib/components/VirtualList.svelte";
	import VirtualGrid from "$lib/components/VirtualGrid.svelte";
	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { Trash2, RotateCcw, List, LayoutGrid } from "lucide-svelte";

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

	const allIds = $derived(items.map((i) => i.id));
	const allSelected = $derived(items.length > 0 && selected.size === items.length);
	const someSelected = $derived(selected.size > 0 && selected.size < items.length);

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

	function toggleItem(id: string) {
		const next = new Set(selected);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selected = next;
		lastSelected = id;
	}

	function toggleSelectAll() {
		if (allSelected) {
			selected = new Set();
		} else {
			selected = new Set(allIds);
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

	async function load() {
		try {
			const res = await listTrash();
			items = res.items;
			trashCount.set(items.length);
		} catch {
			toast.error("Failed to load trash");
		} finally {
			loading = false;
		}
	}

	onMount(() => { load(); });

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
<div class="flex h-full flex-col gap-4 p-4" onclick={clearSelection}>
	<!-- Toolbar -->
	<div class="flex items-center gap-3">
		<div class="flex items-center gap-2">
			<Trash2 class="size-5 text-muted-foreground" strokeWidth={2} />
			<h1 class="text-lg font-bold tracking-[-0.01em]">Trash</h1>
			{#if items.length > 0}
				<span class="font-mono text-[13px] text-muted-foreground">
					{items.length} {items.length === 1 ? "item" : "items"}
				</span>
			{/if}
		</div>

		{#if selected.size > 0}
			<div class="flex items-center gap-1 rounded-lg border border-border-2 bg-card px-2 py-1">
				<span class="font-mono text-[11px] font-medium tracking-[0.02em] text-muted-foreground">
					{selected.size} selected
				</span>
				<Button variant="ghost" size="icon-xs" onclick={(e) => { e.stopPropagation(); handleBulkRestore(); }} title="Restore">
					<RotateCcw class="size-3.5" strokeWidth={2} />
				</Button>
				<Button
					variant="ghost"
					size="icon-xs"
					class="text-muted-foreground hover:text-destructive"
					onclick={(e) => { e.stopPropagation(); confirmBulkDelete(); }}
					title="Delete permanently"
				>
					<Trash2 class="size-3.5" strokeWidth={2} />
				</Button>
			</div>
		{/if}

		{#if items.length > 0}
			<Button variant="destructive" size="sm" onclick={(e) => { e.stopPropagation(); emptyConfirmOpen = true; }}>
				Empty Trash
			</Button>
		{/if}

		<div class="ml-auto inline-flex rounded-lg border border-border-2 bg-card p-[2px]">
			<button
				type="button"
				class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors {preferences.viewMode ===
				'grid'
					? 'bg-muted text-foreground'
					: 'text-muted-foreground hover:text-foreground'}"
				onclick={(e) => { e.stopPropagation(); preferences.viewMode = "grid"; }}
				title="Grid view"
				aria-pressed={preferences.viewMode === "grid"}
			>
				<LayoutGrid class="size-[15px]" strokeWidth={2} />
			</button>
			<button
				type="button"
				class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors {preferences.viewMode ===
				'list'
					? 'bg-muted text-foreground'
					: 'text-muted-foreground hover:text-foreground'}"
				onclick={(e) => { e.stopPropagation(); preferences.viewMode = "list"; }}
				title="List view"
				aria-pressed={preferences.viewMode === "list"}
			>
				<List class="size-[15px]" strokeWidth={2} />
			</button>
		</div>
	</div>

	<!-- Content -->
	{#if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading…
		</div>
	{:else if items.length === 0}
		<div class="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
			<Trash2 class="size-12 opacity-30" strokeWidth={1.5} />
			<p class="text-[15px]">Trash is empty</p>
		</div>
	{:else if preferences.viewMode === "grid"}
		<!-- Grid View -->
		<div class="flex min-h-0 flex-1 flex-col">
			<VirtualGrid items={items}>
				{#snippet cell({ item: raw })}
					{@const item = raw as TrashItem}
					{@const isSelected = selected.has(item.id)}
					{@const lastDot = itemName(item).lastIndexOf(".")}
					{@const ext = !item.isDir && lastDot > 0
						? itemName(item).slice(lastDot + 1, lastDot + 5).toUpperCase()
						: null}
					<ContextMenu.Root>
						<ContextMenu.Trigger>
							{#snippet child({ props })}
								<div
									{...props}
									class="relative flex h-full cursor-pointer flex-col items-center gap-2 rounded-xl border p-3 transition-colors select-none
										{isSelected
											? 'border-accent-brand bg-accent-brand-dim'
											: 'border-border hover:border-border-2 hover:bg-muted'}"
									onclick={(e) => handleItemClick(e, item)}
									role="gridcell"
									tabindex={0}
								>
									<div class="flex flex-1 items-center justify-center">
										<FileIcon
											isDir={item.isDir}
											class="size-12 {item.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
											strokeWidth={1.2}
										/>
									</div>
									<span class="w-full truncate text-center text-sm font-medium">
										{itemName(item)}
									</span>
									<div class="flex items-center gap-1.5 font-mono text-[11px] text-muted-foreground">
										{#if ext}
											<span class="rounded-[5px] bg-muted px-1.5 py-0.5 font-medium tracking-[0.02em]">
												{ext}
											</span>
										{/if}
										{#if !item.isDir}
											<span>{formatFileSize(item.size)}</span>
										{/if}
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
				{/snippet}
			</VirtualGrid>
		</div>
	{:else}
		<!-- List View -->
		<div
			class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-border bg-card"
		>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="hidden border-b border-border font-mono text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid md:grid-cols-[40px_minmax(0,1fr)_220px_160px_110px] md:gap-3 md:px-[14px] md:py-2.5"
				onclick={(e) => e.stopPropagation()}
			>
				<div class="flex items-center">
					<Checkbox
						checked={allSelected}
						indeterminate={!allSelected && someSelected}
						onCheckedChange={toggleSelectAll}
					/>
				</div>
				<div>Name</div>
				<div>Original Location</div>
				<div class="text-right">Deleted</div>
				<div class="text-right">Size</div>
			</div>

			<VirtualList items={items} estimateSize={() => 48}>
				{#snippet row({ item: raw, style })}
					{@const item = raw as TrashItem}
					{@const isSelected = selected.has(item.id)}
					{@const lastDot = itemName(item).lastIndexOf(".")}
					{@const ext = !item.isDir && lastDot > 0
						? itemName(item).slice(lastDot + 1, lastDot + 5).toUpperCase()
						: null}
					{@const parent = item.originalPath.substring(0, item.originalPath.lastIndexOf("/")) || "/"}
					<ContextMenu.Root>
						<ContextMenu.Trigger>
							{#snippet child({ props })}
								<div
									{...props}
									class="grid cursor-pointer items-center border-b border-border transition-colors select-none grid-cols-[1fr_auto] md:grid-cols-[40px_minmax(0,1fr)_220px_160px_110px] md:gap-3 px-[14px] py-3.5 md:py-[11px]
										{isSelected ? 'bg-accent-brand-dim' : 'hover:bg-muted'}"
									{style}
									onclick={(e) => handleItemClick(e, item)}
									role="row"
									tabindex={0}
								>
									<div
										class="hidden items-center md:flex"
										onclick={(e) => e.stopPropagation()}
										role="presentation"
									>
										<Checkbox
											checked={isSelected}
											onCheckedChange={() => toggleItem(item.id)}
										/>
									</div>
									<div class="flex min-w-0 items-center gap-3">
										<FileIcon
											isDir={item.isDir}
											class="size-7 shrink-0 md:size-6 {item.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
											strokeWidth={1.4}
										/>
										<span class="min-w-0 flex-1 truncate text-[15px] font-medium md:text-base">
											{itemName(item)}
										</span>
										{#if ext}
											<span
												class="hidden shrink-0 rounded-[5px] bg-muted px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] text-muted-foreground md:inline-flex"
											>
												{ext}
											</span>
										{/if}
									</div>
									<div class="flex shrink-0 items-center font-mono text-xs text-muted-foreground md:hidden">
										{item.isDir ? "—" : formatFileSize(item.size)}
									</div>
									<div class="hidden truncate font-mono text-[13px] text-muted-foreground md:block">
										{parent}
									</div>
									<div class="hidden text-right font-mono text-[13px] text-muted-foreground md:block">
										{formatDate(item.deletedAt)}
									</div>
									<div class="hidden text-right font-mono text-[13px] text-muted-foreground md:block">
										{item.isDir ? "—" : formatFileSize(item.size)}
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
				{/snippet}
			</VirtualList>
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
