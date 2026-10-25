<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { toast } from "svelte-sonner";
	import { listVersions, restoreVersion, deleteVersion } from "$lib/api/versions.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import LoadingState from "$lib/components/LoadingState.svelte";
	import type { FileVersion } from "$lib/types";
	import { versioningEnabled } from "$lib/stores/versioningEnabled.svelte.js";

	let {
		open = $bindable(false),
		path,
		onrestored,
	}: {
		open: boolean;
		path: string;
		onrestored: () => void;
	} = $props();

	let versions = $state<FileVersion[]>([]);
	let loading = $state(false);
	let busy = $state<{ id: number; action: "restore" | "delete" } | null>(null);
	// Both actions are destructive (restore may have no backup), so each asks inline first
	let confirming = $state<{ id: number; action: "restore" | "delete" } | null>(null);

	// The server snapshots the current file before a restore, but skips files over this cap
	const restoreNote = $derived(
		versioningEnabled.maxFileSize > 0
			? `The current file is saved as a version first, unless it is over ${formatFileSize(versioningEnabled.maxFileSize)}.`
			: "The current file is saved as a version first.",
	);

	async function load() {
		if (!path) return;
		loading = true;
		try {
			const res = await listVersions(path);
			versions = res.items;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to load versions");
			versions = [];
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (open && path) {
			confirming = null;
			load();
		}
	});

	async function handleRestore(v: FileVersion) {
		confirming = null;
		busy = { id: v.id, action: "restore" };
		try {
			await restoreVersion(v.id);
			toast.success("Version restored");
			open = false;
			onrestored();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Restore failed");
		} finally {
			busy = null;
		}
	}

	async function handleDelete(v: FileVersion) {
		confirming = null;
		busy = { id: v.id, action: "delete" };
		try {
			await deleteVersion(v.id);
			versions = versions.filter((x) => x.id !== v.id);
			toast.success("Version deleted");
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Delete failed");
		} finally {
			busy = null;
		}
	}

	const fileName = $derived(path ? path.split("/").pop() ?? path : "");
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Version history</Dialog.Title>
			<Dialog.Description>Previous versions of {fileName}</Dialog.Description>
		</Dialog.Header>

		<div class="min-h-[200px]">
		{#if loading}
			<LoadingState compact />
		{:else if versions.length === 0}
			<p class="py-6 text-center text-sm text-muted-foreground">No previous versions</p>
		{:else}
			<div class="max-h-96 overflow-y-auto rounded-lg border border-border bg-background">
				<ul class="divide-y divide-border">
					{#each versions as v (v.id)}
						<li class="flex items-center justify-between gap-3 px-3 py-2.5">
							<div class="min-w-0 flex-1">
								<p class="truncate text-meta tabular-nums text-foreground">{formatDate(v.createdAt)}</p>
								<p class="text-meta tabular-nums text-muted-foreground">{formatFileSize(v.size)}</p>
								{#if confirming?.id === v.id}
									<p class="mt-1 text-xs text-muted-foreground">
										{confirming.action === "restore" ? `Replace the current file with this version? ${restoreNote}` : "Permanently delete this version? This can't be undone."}
									</p>
								{/if}
							</div>
							<div class="flex shrink-0 gap-2">
								{#if confirming?.id === v.id}
									<Button size="sm" variant="outline" onclick={() => (confirming = null)}>Cancel</Button>
									<Button
										size="sm"
										variant="destructive"
										onclick={() => (confirming?.action === "restore" ? handleRestore(v) : handleDelete(v))}
									>
										{confirming.action === "restore" ? "Restore" : "Delete"}
									</Button>
								{:else}
									<Button
										size="sm"
										variant="outline"
										disabled={busy !== null}
										loading={busy?.id === v.id && busy?.action === "restore"}
										onclick={() => (confirming = { id: v.id, action: "restore" })}
									>
										Restore
									</Button>
									<Button
										size="sm"
										variant="destructive"
										disabled={busy !== null}
										loading={busy?.id === v.id && busy?.action === "delete"}
										onclick={() => (confirming = { id: v.id, action: "delete" })}
									>
										Delete
									</Button>
								{/if}
							</div>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Close</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
