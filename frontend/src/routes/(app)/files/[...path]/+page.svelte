<script lang="ts">
	import { page } from "$app/state";
	import { listDirectory } from "$lib/api/files.js";
	import type { DirectoryListing } from "$lib/types";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileList from "$lib/components/FileList.svelte";

	const path = $derived(page.params.path ?? "");

	let listing = $state<DirectoryListing | null>(null);
	let error = $state<string | null>(null);
	let loading = $state(true);

	async function load(dirPath: string) {
		loading = true;
		error = null;
		try {
			listing = await listDirectory(dirPath);
		} catch (e) {
			error = e instanceof Error ? e.message : "Failed to load directory";
			listing = null;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load(path);
	});
</script>

<div class="flex flex-col gap-4 p-4">
	<Breadcrumbs {path} />

	{#if loading}
		<div class="flex items-center justify-center py-20 text-sm text-muted-foreground">
			Loading...
		</div>
	{:else if error}
		<div class="flex items-center justify-center py-20 text-sm text-destructive">
			{error}
		</div>
	{:else if listing}
		<FileList items={listing.items} />
	{/if}
</div>
