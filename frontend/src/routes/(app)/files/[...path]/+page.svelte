<script lang="ts">
	import { page } from "$app/state";
	import { listDirectory } from "$lib/api/files.js";
	import type { DirectoryListing, FileInfo } from "$lib/types";
	import type { SortField, SortDir, ViewMode } from "$lib/stores/preferences.svelte.js";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileList from "$lib/components/FileList.svelte";
	import FileGrid from "$lib/components/FileGrid.svelte";
	import ViewControls from "$lib/components/ViewControls.svelte";

	const path = $derived(page.params.path ?? "");

	let listing = $state<DirectoryListing | null>(null);
	let error = $state<string | null>(null);
	let loading = $state(true);

	async function load(dirPath: string, showHidden: boolean) {
		loading = true;
		error = null;
		try {
			listing = await listDirectory(dirPath, showHidden);
		} catch (e) {
			error = e instanceof Error ? e.message : "Failed to load directory";
			listing = null;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load(path, preferences.showHidden);
	});

	function compareItems(a: FileInfo, b: FileInfo, field: SortField, dir: SortDir): number {
		let cmp = 0;
		switch (field) {
			case "name":
				cmp = a.name.localeCompare(b.name);
				break;
			case "size":
				cmp = a.size - b.size;
				break;
			case "modified":
				cmp = a.modTime - b.modTime;
				break;
			case "type":
				cmp = (a.mimeType ?? "").localeCompare(b.mimeType ?? "");
				break;
		}
		return dir === "asc" ? cmp : -cmp;
	}

	const sorted = $derived.by(() => {
		if (!listing) return [];
		const dirs = listing.items.filter((f) => f.isDir);
		const files = listing.items.filter((f) => !f.isDir);
		const { sortField, sortDir } = preferences;
		dirs.sort((a, b) => compareItems(a, b, sortField, sortDir));
		files.sort((a, b) => compareItems(a, b, sortField, sortDir));
		return [...dirs, ...files];
	});

	// Smart view: auto-grid if >50% of items are image/video
	const smartView = $derived.by((): ViewMode => {
		if (!listing || listing.items.length === 0) return preferences.viewMode;
		const mediaCount = listing.items.filter((f) => {
			const mime = f.mimeType ?? "";
			return mime.startsWith("image/") || mime.startsWith("video/");
		}).length;
		return mediaCount / listing.items.length > 0.5 ? "grid" : preferences.viewMode;
	});

	const activeView = $derived(preferences.getDirectoryOverride(path) ?? smartView);

	function handleViewChange(mode: ViewMode) {
		if (mode !== smartView) {
			preferences.setDirectoryViewMode(path, mode);
		} else {
			preferences.clearDirectoryOverride(path);
		}
		preferences.viewMode = mode;
	}
</script>

<div class="flex h-full flex-col gap-4 p-4">
	<Breadcrumbs {path} />

	{#if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading...
		</div>
	{:else if error}
		<div class="flex items-center justify-center py-20 text-sm text-destructive">
			{error}
		</div>
	{:else}
		<ViewControls viewMode={activeView} onviewchange={handleViewChange} />

		<div class="flex min-h-0 flex-1 flex-col">
			{#if activeView === "grid"}
				<FileGrid items={sorted} />
			{:else}
				<FileList items={sorted} />
			{/if}
		</div>
	{/if}
</div>
