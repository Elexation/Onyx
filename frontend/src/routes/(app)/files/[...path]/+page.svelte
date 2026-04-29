<script lang="ts">
	import { page } from "$app/state";
	import { goto } from "$app/navigation";
	import { listDirectory, getDownloadUrl, getZipDownloadUrl, move } from "$lib/api/files.js";
	import { checkConflicts } from "$lib/api/upload.js";
	import { getSettings } from "$lib/api/settings.js";
	import type { DirectoryListing, FileInfo } from "$lib/types";
	import type { SortField, SortDir, ViewMode } from "$lib/stores/preferences.svelte.js";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { uploadState } from "$lib/stores/upload.svelte.js";
	import { trashCount } from "$lib/stores/trashCount.svelte.js";
	import { addFiles, startUpload, getUppy } from "$lib/upload/uppy.js";
	import { shortcuts, type ShortcutMap } from "$lib/actions/keyboard.js";
	import { toast } from "svelte-sonner";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileList from "$lib/components/FileList.svelte";
	import FileGrid from "$lib/components/FileGrid.svelte";
	import FileToolbar from "$lib/components/FileToolbar.svelte";
	import ViewControls from "$lib/components/ViewControls.svelte";
	import UploadZone from "$lib/components/UploadZone.svelte";
	import RenameDialog from "$lib/components/dialogs/RenameDialog.svelte";
	import NewFolderDialog from "$lib/components/dialogs/NewFolderDialog.svelte";
	import DeleteDialog from "$lib/components/dialogs/DeleteDialog.svelte";
	import MoveDialog from "$lib/components/dialogs/MoveDialog.svelte";
	import ConflictDialog from "$lib/components/dialogs/ConflictDialog.svelte";
	import VersionHistoryDialog from "$lib/components/dialogs/VersionHistoryDialog.svelte";
	import PreviewModal from "$lib/components/preview/PreviewModal.svelte";
	import { canPreview } from "$lib/preview.js";

	const path = $derived(page.params.path ?? "");

	let listing = $state<DirectoryListing | null>(null);
	let error = $state<string | null>(null);
	let loading = $state(true);

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

	// Trash setting
	let trashEnabled = $state(true);
	getSettings().then((s) => { trashEnabled = s["trash.enabled"] !== "false"; }).catch(() => {});

	// Upload state
	let conflictOpen = $state(false);
	let conflictNames = $state<string[]>([]);
	let pendingUploadFiles = $state<File[]>([]);
	let pendingDropFileIds = $state<string[]>([]);

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
			path: parts.slice(0, -1).join("/"),
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

	function refresh() {
		load(path, preferences.showHidden);
	}

	// Actions
	function handleOpen(item: FileInfo) {
		if (item.isDir) {
			goto(`/files/${item.path}`);
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
		const targetDir = path || "/";
		const relativePaths = files.map(
			(f) => (f as any).webkitRelativePath || f.name,
		);

		try {
			const { conflicts } = await checkConflicts(targetDir, relativePaths);
			if (conflicts.length > 0) {
				pendingUploadFiles = files;
				conflictNames = conflicts;
				conflictOpen = true;
			} else {
				addFiles(files, targetDir);
				startUpload();
			}
		} catch {
			// If conflict check fails, upload anyway without conflict resolution
			addFiles(files, targetDir);
			startUpload();
		}
	}

	function handleConflictResolve(resolutions: Record<string, "replace" | "keepBoth" | "skip">) {
		conflictOpen = false;
		const targetDir = path || "/";

		if (pendingDropFileIds.length > 0) {
			// Resolve conflicts for files already in Uppy (from drag-and-drop)
			const uppy = getUppy();
			for (const fileId of pendingDropFileIds) {
				const file = uppy.getFile(fileId);
				if (!file) continue;
				const rp = (file.meta as any).relativePath || file.name;
				const resolution = resolutions[rp];
				if (resolution === "skip") {
					uppy.removeFile(fileId);
				} else if (resolution) {
					uppy.setFileMeta(fileId, { conflictStrategy: resolution });
				}
			}
			pendingDropFileIds = [];
			startUpload();
		} else {
			// Resolve conflicts for files from the picker button
			addFiles(pendingUploadFiles, targetDir, resolutions);
			startUpload();
			pendingUploadFiles = [];
		}
	}

	function handleDropConflicts(fileIds: string[], conflicts: string[]) {
		pendingDropFileIds = fileIds;
		conflictNames = conflicts;
		conflictOpen = true;
	}

	// Refresh file list when uploads complete
	// Small delay: tus signals completion to the client before the server
	// finishes moving the file from the upload store to the data directory.
	$effect(() => {
		const uppy = getUppy();
		let timer: ReturnType<typeof setTimeout>;
		const handler = () => {
			timer = setTimeout(() => refresh(), 500);
		};
		uppy.on("complete", handler);
		return () => {
			clearTimeout(timer);
			uppy.off("complete", handler);
		};
	});

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
	class="flex h-full flex-col gap-4 p-4"
	tabindex={0}
	role="application"
	use:shortcuts={shortcutMap}
>
	<Breadcrumbs {path} ondrop={handleDrop} />

	{#if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading...
		</div>
	{:else if error}
		<div class="flex items-center justify-center py-20 text-sm text-destructive">
			{error}
		</div>
	{:else}
		<FileToolbar
			onnewfolder={() => (newFolderOpen = true)}
			ondelete={() => handleDelete([...selection.items])}
			onpaste={handlePaste}
			oncopy={handleCopy}
			oncut={handleCut}
			ondownload={handleDownload}
			onupload={handleUpload}
		>
			{#snippet viewControls()}
				<ViewControls viewMode={activeView} onviewchange={handleViewChange} />
			{/snippet}
		</FileToolbar>

		<UploadZone currentDir={path || "/"} onconflicts={handleDropConflicts}>
			<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
			<div class="flex min-h-0 flex-1 flex-col" onclick={() => selection.clear()}>
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
						ondrop={handleDrop}
					/>
				{/if}
			</div>
		</UploadZone>
	{/if}
</div>

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
	{trashEnabled}
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
		conflicts={conflictNames}
		onresolve={handleConflictResolve}
	/>
{/if}

<VersionHistoryDialog
	bind:open={versionHistoryOpen}
	path={versionHistoryPath}
	onrestored={refresh}
/>

{#if previewOpen && previewFile}
	<PreviewModal
		bind:file={previewFile}
		items={listing?.items ?? []}
		onclose={() => { previewOpen = false; previewFile = null; }}
	/>
{/if}
