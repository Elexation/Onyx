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
	import { Trash2, RotateCcw, X, List, LayoutGrid } from "lucide-svelte";

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
	<div class="flex items-center gap-2">
		<div class="flex items-center gap-2">
			<Trash2 class="size-5 text-muted-foreground" />
			<h1 class="text-lg font-semibold">Trash</h1>
			{#if items.length > 0}
				<span class="text-sm text-muted-foreground">({items.length} {items.length === 1 ? "item" : "items"})</span>
			{/if}
		</div>

		{#if selected.size > 0}
			<div class="flex items-center gap-1 rounded-md border border-border px-2 py-1">
				<span class="text-xs text-muted-foreground">{selected.size} selected</span>
				<Button variant="ghost" size="icon-xs" onclick={(e) => { e.stopPropagation(); handleBulkRestore(); }} title="Restore">
					<RotateCcw class="size-3.5" />
				</Button>
				<Button variant="ghost" size="icon-xs" onclick={(e) => { e.stopPropagation(); confirmBulkDelete(); }} title="Delete permanently">
					<X class="size-3.5 text-destructive" />
				</Button>
			</div>
		{/if}

		{#if items.length > 0}
			<Button variant="destructive" size="sm" onclick={(e) => { e.stopPropagation(); emptyConfirmOpen = true; }}>
				Empty Trash
			</Button>
		{/if}

		<div class="ml-auto flex items-center gap-1 rounded-md border border-border p-0.5">
			<button
				class="rounded p-1 transition-colors {preferences.viewMode === 'list' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}"
				onclick={(e) => { e.stopPropagation(); preferences.viewMode = "list"; }}
				title="List view"
			>
				<List class="size-4" />
			</button>
			<button
				class="rounded p-1 transition-colors {preferences.viewMode === 'grid' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground'}"
				onclick={(e) => { e.stopPropagation(); preferences.viewMode = "grid"; }}
				title="Grid view"
			>
				<LayoutGrid class="size-4" />
			</button>
		</div>
	</div>

	<!-- Content -->
	{#if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading...
		</div>
	{:else if items.length === 0}
		<div class="flex flex-col items-center justify-center gap-2 py-24 text-muted-foreground">
			<Trash2 class="size-12 opacity-30" />
			<p>Trash is empty</p>
		</div>
	{:else if preferences.viewMode === "grid"}
		<!-- Grid View -->
		<div class="flex min-h-0 flex-1 flex-col">
			<VirtualGrid items={items}>
				{#snippet cell({ item: raw })}
					{@const item = raw as TrashItem}
					{@const isSelected = selected.has(item.id)}
					<ContextMenu.Root>
						<ContextMenu.Trigger>
							{#snippet child({ props })}
								<div
									{...props}
									class="relative flex cursor-pointer flex-col items-center gap-2 rounded-lg border border-border/50 p-3 transition-colors select-none
										{isSelected ? 'bg-accent/70 border-accent' : 'hover:bg-accent/50'}"
									onclick={(e) => handleItemClick(e, item)}
									role="gridcell"
									tabindex={0}
								>
									<FileIcon isDir={item.isDir} class="size-10 text-muted-foreground" />
									<span class="w-full truncate text-center text-xs">{itemName(item)}</span>
									{#if !item.isDir}
										<span class="text-[10px] text-muted-foreground">{formatFileSize(item.size)}</span>
									{/if}
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
		<div class="flex min-h-0 flex-1 flex-col">
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="flex border-b border-border text-xs text-muted-foreground" onclick={(e) => e.stopPropagation()}>
				<div class="flex w-10 items-center py-2 pl-4">
					<Checkbox
						checked={allSelected ? true : someSelected ? "indeterminate" : false}
						onCheckedChange={toggleSelectAll}
					/>
				</div>
				<div class="flex-1 py-2 font-medium">Name</div>
				<div class="w-44 py-2 pr-4 font-medium">Original Location</div>
				<div class="w-36 py-2 pr-4 text-right font-medium">Deleted</div>
				<div class="w-24 py-2 pr-4 text-right font-medium">Size</div>
			</div>

			<VirtualList items={items} estimateSize={() => 41}>
				{#snippet row({ item: raw, style })}
					{@const item = raw as TrashItem}
					{@const isSelected = selected.has(item.id)}
					<ContextMenu.Root>
						<ContextMenu.Trigger>
							{#snippet child({ props })}
								<div
									{...props}
									class="group flex cursor-pointer items-center border-b border-border/50 transition-colors select-none
										{isSelected ? 'bg-accent/70' : 'hover:bg-accent/50'}"
									{style}
									onclick={(e) => handleItemClick(e, item)}
									role="row"
									tabindex={0}
								>
									<div class="flex w-10 items-center py-2 pl-4" onclick={(e) => e.stopPropagation()}>
										<Checkbox
											checked={isSelected}
											onCheckedChange={() => toggleItem(item.id)}
										/>
									</div>
									<div class="flex flex-1 items-center gap-2 py-2 text-sm">
										<FileIcon isDir={item.isDir} />
										{itemName(item)}
									</div>
									<div class="w-44 py-2 pr-4 text-sm text-muted-foreground">
										{item.originalPath.substring(0, item.originalPath.lastIndexOf("/")) || "/"}
									</div>
									<div class="w-36 py-2 pr-4 text-right text-sm text-muted-foreground">
										{formatDate(item.deletedAt)}
									</div>
									<div class="w-24 py-2 pr-4 text-right text-sm text-muted-foreground">
										{formatFileSize(item.size)}
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
