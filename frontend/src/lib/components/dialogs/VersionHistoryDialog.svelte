<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { toast } from "svelte-sonner";
	import { listVersions, restoreVersion, deleteVersion } from "$lib/api/versions.js";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import type { FileVersion } from "$lib/types";

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
	let busy = $state<number | null>(null);

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
			load();
		}
	});

	async function handleRestore(v: FileVersion) {
		busy = v.id;
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
		busy = v.id;
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
	<Dialog.Content class="max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Version history</Dialog.Title>
			<Dialog.Description>Previous versions of {fileName}</Dialog.Description>
		</Dialog.Header>

		{#if loading}
			<p class="py-6 text-center text-sm text-muted-foreground">Loading…</p>
		{:else if versions.length === 0}
			<p class="py-6 text-center text-sm text-muted-foreground">No previous versions</p>
		{:else}
			<div class="max-h-96 overflow-y-auto">
				<ul class="divide-y divide-border">
					{#each versions as v (v.id)}
						<li class="flex items-center justify-between gap-2 py-3">
							<div class="min-w-0 flex-1">
								<p class="truncate text-sm">{formatDate(v.createdAt)}</p>
								<p class="text-xs text-muted-foreground">{formatFileSize(v.size)}</p>
							</div>
							<div class="flex shrink-0 gap-2">
								<Button
									size="sm"
									variant="outline"
									disabled={busy !== null}
									onclick={() => handleRestore(v)}
								>
									Restore
								</Button>
								<Button
									size="sm"
									variant="ghost"
									disabled={busy !== null}
									onclick={() => handleDelete(v)}
								>
									Delete
								</Button>
							</div>
						</li>
					{/each}
				</ul>
			</div>
		{/if}

		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Close</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
