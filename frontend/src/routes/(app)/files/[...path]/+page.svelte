<script lang="ts">
	import { untrack } from "svelte";
	import { page } from "$app/state";
	import { goto } from "$app/navigation";
	import { listDirectory, getDownloadUrl, getZipDownloadUrl, move } from "$lib/api/files.js";
	import { checkConflicts } from "$lib/api/upload.js";
	import type { DirectoryListing, FileInfo } from "$lib/types";
	import type { SortField, SortDir, ViewMode } from "$lib/stores/preferences.svelte.js";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";

	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { trashEnabled } from "$lib/stores/trashEnabled.svelte.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { sharedPaths } from "$lib/stores/sharedPaths.svelte.js";
	import { viewport } from "$lib/stores/viewport.svelte.js";
	import { addFiles, startUpload } from "$lib/upload/uppy.js";
	import { changes } from "$lib/changes";
	import { shortcuts, type ShortcutMap } from "$lib/actions/keyboard.js";
	import { toast } from "svelte-sonner";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileList from "$lib/components/FileList.svelte";
	import FileGrid from "$lib/components/FileGrid.svelte";
	import FileToolbar from "$lib/components/FileToolbar.svelte";
	import ViewControls from "$lib/components/ViewControls.svelte";
	import UploadZone from "$lib/components/UploadZone.svelte";
	import MobileFAB from "$lib/components/MobileFAB.svelte";
	import RenameDialog from "$lib/components/dialogs/RenameDialog.svelte";
	import NewFolderDialog from "$lib/components/dialogs/NewFolderDialog.svelte";
	import DeleteDialog from "$lib/components/dialogs/DeleteDialog.svelte";
	import MoveDialog from "$lib/components/dialogs/MoveDialog.svelte";
	import ConflictDialog, { type ConflictPair } from "$lib/components/dialogs/ConflictDialog.svelte";
	import VersionHistoryDialog from "$lib/components/dialogs/VersionHistoryDialog.svelte";
	import ShareDialog from "$lib/components/dialogs/ShareDialog.svelte";
	import PreviewModal from "$lib/components/preview/PreviewModal.svelte";
	import { canPreview } from "$lib/preview.js";

	const path = $derived(page.params.path ?? "");

	let listing = $state<DirectoryListing | null>(null);
	let error = $state<string | null>(null);
	let loading = $state(true);
	let showLoading = $state(false);
	let refreshing = $state(false);

	// Dialog state
	let renameOpen = $state(false);
	let renameTarget = $state<FileInfo | null>(null);
	let newFolderOpen = $state(false);
	let deleteOpen = $state(false);
	let deletePaths = $state<string[]>([]);
	let moveOpen = $state(false);
	let movePaths = $state<string[]>([]);
	let moveMode = $state<"move" | "copy">("move");
	let versionHistoryOpen = $state(false);
	let versionHistoryPath = $state("");
	let previewOpen = $state(false);
	let previewFile = $state<FileInfo | null>(null);
	let shareOpen = $state(false);
	let shareTarget = $state<FileInfo | null>(null);

	// Background context menu state
	let bgMenuOpen = $state(false);
	let bgMenuPos = $state({ x: 0, y: 0 });

	function handleBgContextMenu(e: MouseEvent) {
		if (e.defaultPrevented) return;
		e.preventDefault();
		if (viewport.isMobile) return;
		bgMenuPos = { x: e.clientX, y: e.clientY };
		bgMenuOpen = true;
	}

	// Upload state
	let conflictOpen = $state(false);
	let conflictPairs = $state<ConflictPair[]>([]);
	let pendingUploadFiles = $state<File[]>([]);

	function handleShareSelected() {
		if (selection.count !== 1) return;
		const p = [...selection.items][0];
		const item = sorted.find((i) => i.path === p);
		if (item) handleShare(item);
	}

	async function load(
		dirPath: string,
		opts: { isRefresh?: boolean; isCancelled?: () => boolean } = {},
	) {
		const { isRefresh = false, isCancelled } = opts;
		if (!isRefresh) {
			loading = true;
			error = null;
		}
		try {
			const result = await listDirectory(dirPath);
			if (isCancelled?.()) return;
			listing = result;
			if (isRefresh) error = null;
		} catch (e) {
			if (isCancelled?.()) return;
			const msg = e instanceof Error ? e.message : "Failed to load directory";
			if (isRefresh) {
				toast.error(msg);
			} else {
				error = msg;
				listing = null;
			}
		} finally {
			if (!isCancelled?.() && !isRefresh) loading = false;
		}
	}

	// Delay-gate the Loading… UI so fast loads don't flash.
	$effect(() => {
		if (!loading) {
			showLoading = false;
			return;
		}
		const t = setTimeout(() => {
			showLoading = true;
		}, 250);
		return () => clearTimeout(t);
	});

	let navGen = 0;
	$effect(() => {
		const myGen = ++navGen;
		load(path, { isCancelled: () => myGen !== navGen });
		if (untrack(() => sharesEnabled.enabled)) sharedPaths.refresh();
	});

	// Clear selection on navigation
	$effect(() => {
		path;
		selection.clear();
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

	const parentEntry = $derived.by((): FileInfo | null => {
		if (!path) return null;
		const parts = path.split("/").filter(Boolean);
		return {
			name: "..",
			path: "/" + parts.slice(0, -1).join("/"),
			isDir: true,
			size: 0,
			modTime: 0,
		};
	});

	const sorted = $derived.by(() => {
		if (!listing) return [];
		const dirs = listing.items.filter((f) => f.isDir);
		const files = listing.items.filter((f) => !f.isDir);
		const { sortField, sortDir } = preferences;
		dirs.sort((a, b) => compareItems(a, b, sortField, sortDir));
		files.sort((a, b) => compareItems(a, b, sortField, sortDir));
		const result = [...dirs, ...files];
		if (parentEntry) result.unshift(parentEntry);
		return result;
	});

	const allPaths = $derived(sorted.filter((i) => i.name !== "..").map((i) => i.path));

	const activeView = $derived(preferences.viewMode);

	function handleViewChange(mode: ViewMode) {
		preferences.viewMode = mode;
	}

	async function refresh() {
		const myGen = navGen;
		refreshing = true;
		try {
			await Promise.all([
				load(path, { isRefresh: true, isCancelled: () => myGen !== navGen }),
				new Promise((r) => setTimeout(r, 400)),
			]);
		} finally {
			refreshing = false;
		}
	}

	// Actions
	function handleOpen(item: FileInfo) {
		if (item.isDir) {
			goto(`/files${item.path}`);
		} else if (canPreview(item)) {
			previewFile = item;
			previewOpen = true;
		} else {
			const a = document.createElement("a");
			a.href = getDownloadUrl(item.path);
			a.download = item.name;
			a.click();
		}
	}

	function handleDownload() {
		const paths = selection.count > 0 ? [...selection.items] : [];
		if (paths.length === 0) return;

		if (paths.length === 1) {
			const item = sorted.find((i) => i.path === paths[0]);
			if (item && !item.isDir) {
				const a = document.createElement("a");
				a.href = getDownloadUrl(item.path);
				a.download = item.name;
				a.click();
				return;
			}
		}

		const a = document.createElement("a");
		a.href = getZipDownloadUrl(paths);
		a.download = "";
		a.click();
	}

	function handleRename(item: FileInfo) {
		renameTarget = item;
		renameOpen = true;
	}

	function handleDelete(paths: string[]) {
		deletePaths = paths;
		deleteOpen = true;
	}

	function handleMoveTo(paths: string[]) {
		movePaths = paths;
		moveMode = "move";
		moveOpen = true;
	}

	function handleCopyTo(paths: string[]) {
		movePaths = paths;
		moveMode = "copy";
		moveOpen = true;
	}

	function handleVersions(item: FileInfo) {
		versionHistoryPath = item.path;
		versionHistoryOpen = true;
	}

	function handleShare(item: FileInfo) {
		if (!sharesEnabled.enabled) return;
		shareTarget = item;
		shareOpen = true;
	}

	async function handlePaste() {
		if (!clipboard.hasItems) return;
		try {
			const results = await clipboard.paste(path || "/");
			const failed = results.filter((r) => !r.success);
			if (failed.length === 0) {
				toast.success(results.length === 1 ? "Pasted item" : `Pasted ${results.length} items`);
			} else {
				toast.error(`${failed.length} item(s) failed`);
			}
			selection.clear();
			refresh();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Paste failed");
		}
	}

	function handleCopy() {
		const paths = selection.count > 0 ? [...selection.items] : [];
		if (paths.length === 0) return;
		clipboard.copy(paths);
		toast.success(paths.length === 1 ? "Copied to clipboard" : `Copied ${paths.length} items`);
	}

	function handleCut() {
		const paths = selection.count > 0 ? [...selection.items] : [];
		if (paths.length === 0) return;
		clipboard.cut(paths);
		toast.success(paths.length === 1 ? "Cut to clipboard" : `Cut ${paths.length} items`);
	}

	function handleDeleteSuccess() {
		selection.clear();
		refresh();
		trashCount.refresh();
	}

	function handleRenameSuccess() {
		selection.clear();
		refresh();
	}

	function handleMoveSuccess() {
		selection.clear();
		refresh();
	}

	async function handleDrop(paths: string[], destination: string) {
		try {
			const dest = destination || "/";
			const { results } = await move(paths, dest);
			const failed = results.filter((r) => !r.success);
			if (failed.length === 0) {
				toast.success(
					results.length === 1
						? `Moved item to ${dest}`
						: `Moved ${results.length} items to ${dest}`,
				);
			} else {
				toast.error(`${failed.length} item(s) failed to move`);
			}
			selection.clear();
			refresh();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Move failed");
		}
	}

	// Upload handling
	async function handleUpload(files: File[]) {
		if (conflictOpen) return;
		const targetDir = path || "/";
		const relativePaths = files.map(
			(f) => (f as any).webkitRelativePath || (f as any).relativePath || f.name,
		);

		try {
			const { conflicts } = await checkConflicts(targetDir, relativePaths);
			if (conflicts.length > 0) {
				const incomingByPath = new Map<string, File>();
				files.forEach((f, i) => incomingByPath.set(relativePaths[i], f));
				pendingUploadFiles = files;
				conflictPairs = conflicts.map((c) => {
					const f = incomingByPath.get(c.path);
					return {
						path: c.path,
						existing: { size: c.size, modTime: c.modTime },
						incoming: {
							size: f?.size ?? 0,
							modTime: Math.floor((f?.lastModified ?? 0) / 1000),
						},
					};
				});
				conflictOpen = true;
			} else {
				await addFiles(files, targetDir);
				startUpload().catch(() => {});
			}
		} catch {
			// If conflict check fails, upload anyway without conflict resolution
			await addFiles(files, targetDir);
			startUpload().catch(() => {});
		}
	}

	async function handleConflictResolve(resolutions: Record<string, "replace" | "keepBoth" | "skip">) {
		conflictOpen = false;
		const targetDir = path || "/";
		const filesToUpload = pendingUploadFiles;
		pendingUploadFiles = [];
		await addFiles(filesToUpload, targetDir, resolutions);
		startUpload().catch(() => {});
	}

	// Live updates: refetch this directory's listing when the server emits
	// a relevant event. Replaces the prior setTimeout(load, 500) hack tied
	// to uppy 'complete' — server now emits file.changed after CompleteUpload's
	// rename completes, so the next 5s poll picks it up deterministically.
	$effect(() => {
		const dir = normalizeDir(path);
		const isInDir = (parent: string) => parent === dir;
		const refetch = () => load(path);

		const offFile = changes.on("file.changed", (p) => {
			if (isInDir(p.parentPath)) refetch();
		});
		const offThumb = changes.on("thumb.ready", (p) => {
			const parent = p.path.substring(0, p.path.lastIndexOf("/")) || "/";
			if (isInDir(parent)) refetch();
		});
		const offBehind = changes.onBehind(refetch);
		return () => {
			offFile();
			offThumb();
			offBehind();
		};
	});

	// Convert SvelteKit's path param ("foo/bar" or "") into the server-side
	// form ("/foo/bar" or "/") used in event payloads' parentPath.
	function normalizeDir(d: string): string {
		if (!d) return "/";
		const n = d.startsWith("/") ? d : "/" + d;
		if (n.length > 1 && n.endsWith("/")) return n.slice(0, -1);
		return n;
	}

	// Keyboard shortcuts
	const shortcutMap: ShortcutMap = {
		"delete": () => {
			if (selection.count > 0) handleDelete([...selection.items]);
		},
		"f2": () => {
			if (selection.count === 1) {
				const p = [...selection.items][0];
				const item = sorted.find((i) => i.path === p);
				if (item) handleRename(item);
			}
		},
		"ctrl+c": () => handleCopy(),
		"ctrl+x": () => handleCut(),
		"ctrl+v": () => handlePaste(),
		"ctrl+a": () => selection.selectAll(allPaths),
		"enter": () => {
			if (selection.count === 1) {
				const p = [...selection.items][0];
				const item = sorted.find((i) => i.path === p);
				if (item) handleOpen(item);
			}
		},
		"escape": () => {
			if (bgMenuOpen) {
				bgMenuOpen = false;
				return;
			}
			if (previewOpen) {
				previewOpen = false;
				previewFile = null;
				return;
			}
			selection.clear();
		},
		"backspace": () => {
			const parts = path.split("/").filter(Boolean);
			if (parts.length > 0) {
				goto(`/files/${parts.slice(0, -1).join("/")}`);
			} else {
				goto("/files");
			}
		},
	};
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
	class="flex h-full flex-col"
	tabindex={0}
	role="application"
	use:shortcuts={shortcutMap}
>
	{#if !showLoading && !error}
		<div class="border-b border-border px-4 py-3">
			<FileToolbar
				onnewfolder={() => (newFolderOpen = true)}
				onrefresh={refresh}
				{refreshing}
				ondelete={() => handleDelete([...selection.items])}
				onpaste={handlePaste}
				ondownload={handleDownload}
				onshare={handleShareSelected}
				onupload={handleUpload}
			>
				{#snippet viewControls()}
					<ViewControls viewMode={activeView} onviewchange={handleViewChange} />
				{/snippet}
			</FileToolbar>
		</div>
	{/if}

	<div class="border-b border-border px-4 py-3">
		<Breadcrumbs {path} ondrop={handleDrop} />
	</div>

	{#if showLoading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading...
		</div>
	{:else if error}
		<div class="flex items-center justify-center py-20 text-sm text-destructive">
			{error}
		</div>
	{:else}
		<UploadZone currentDir={path || "/"} onupload={handleUpload}>
			<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
			<div class="flex min-h-0 flex-1 flex-col p-4" onclick={() => { selection.clear(); bgMenuOpen = false; }} oncontextmenu={handleBgContextMenu}>
				{#if activeView === "grid"}
					<FileGrid
						items={sorted}
						onopen={handleOpen}
						onrename={handleRename}
						ondelete={handleDelete}
						onpaste={handlePaste}
						onmoveto={handleMoveTo}
						oncopyto={handleCopyTo}
						onversions={handleVersions}
						onshare={handleShare}
						ondrop={handleDrop}
					/>
				{:else}
					<FileList
						items={sorted}
						onopen={handleOpen}
						onrename={handleRename}
						ondelete={handleDelete}
						onpaste={handlePaste}
						onmoveto={handleMoveTo}
						oncopyto={handleCopyTo}
						onversions={handleVersions}
						onshare={handleShare}
						ondrop={handleDrop}
					/>
				{/if}
			</div>
		</UploadZone>
	{/if}

	<!-- Mobile-only floating UI -->
	{#if selection.count === 0}
		<MobileFAB
			onnewfolder={() => (newFolderOpen = true)}
			onfiles={handleUpload}
		/>
	{/if}
</div>

{#if bgMenuOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
	<div class="fixed inset-0 z-50" onclick={() => (bgMenuOpen = false)} oncontextmenu={(e) => { e.preventDefault(); bgMenuOpen = false; }}></div>
	<div
		class="fixed z-50 min-w-36 rounded-lg bg-popover p-1 text-popover-foreground ring-1 ring-foreground/10"
		style="left: {bgMenuPos.x}px; top: {bgMenuPos.y}px"
	>
		<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
		<div
			class="flex w-full cursor-default items-center gap-1.5 rounded-md px-1.5 py-1 text-sm select-none hover:bg-accent hover:text-accent-foreground"
			onclick={() => { bgMenuOpen = false; newFolderOpen = true; }}
		>
			New Folder
		</div>
		{#if clipboard.hasItems}
			<div class="my-1 h-px bg-border"></div>
			<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
			<div
				class="flex w-full cursor-default items-center gap-1.5 rounded-md px-1.5 py-1 text-sm select-none hover:bg-accent hover:text-accent-foreground"
				onclick={() => { bgMenuOpen = false; handlePaste(); }}
			>
				Paste
			</div>
		{/if}
	</div>
{/if}

{#if renameTarget}
	<RenameDialog
		bind:open={renameOpen}
		path={renameTarget.path}
		name={renameTarget.name}
		onsuccess={handleRenameSuccess}
	/>
{/if}

<NewFolderDialog
	bind:open={newFolderOpen}
	parentPath={path}
	onsuccess={refresh}
/>

<DeleteDialog
	bind:open={deleteOpen}
	paths={deletePaths}
	trashEnabled={trashEnabled.enabled}
	onsuccess={handleDeleteSuccess}
/>

<MoveDialog
	bind:open={moveOpen}
	paths={movePaths}
	mode={moveMode}
	onsuccess={handleMoveSuccess}
/>

{#if conflictOpen}
	<ConflictDialog
		conflicts={conflictPairs}
		onresolve={handleConflictResolve}
	/>
{/if}

<VersionHistoryDialog
	bind:open={versionHistoryOpen}
	path={versionHistoryPath}
	onrestored={refresh}
/>

{#if shareTarget}
	<ShareDialog
		bind:open={shareOpen}
		path={shareTarget.path}
		isDir={shareTarget.isDir}
	/>
{/if}

{#if previewOpen && previewFile}
	<PreviewModal
		bind:file={previewFile}
		items={listing?.items ?? []}
		onclose={() => { previewOpen = false; previewFile = null; }}
	/>
{/if}
