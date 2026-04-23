<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { deleteFiles } from "$lib/api/files.js";
	import { toast } from "svelte-sonner";

	let {
		open = $bindable(false),
		paths,
		trashEnabled = true,
		onsuccess,
	}: {
		open: boolean;
		paths: string[];
		trashEnabled?: boolean;
		onsuccess: () => void;
	} = $props();

	let submitting = $state(false);

	const itemName = $derived(
		paths.length === 1 ? `"${paths[0].split("/").pop()}"` : `${paths.length} items`
	);

	async function doDelete(permanent: boolean) {
		submitting = true;
		try {
			const res = await deleteFiles(paths, permanent);
			const failed = res.results.filter((r) => !r.success);
			if (failed.length === 0) {
				const msg = permanent
					? paths.length === 1 ? "Deleted" : `Deleted ${paths.length} items`
					: paths.length === 1 ? "Moved to trash" : `Moved ${paths.length} items to trash`;
				toast.success(msg);
			} else {
				toast.error(`${failed.length} item(s) failed to delete`);
			}
			open = false;
			onsuccess();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Delete failed");
		} finally {
			submitting = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			{#if trashEnabled}
				<AlertDialog.Title>Delete {itemName}?</AlertDialog.Title>
				<AlertDialog.Description>Move to trash, or delete permanently?</AlertDialog.Description>
			{:else}
				<AlertDialog.Title>Delete {itemName}?</AlertDialog.Title>
				<AlertDialog.Description>This action cannot be undone.</AlertDialog.Description>
			{/if}
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			{#if trashEnabled}
				<Button variant="destructive" disabled={submitting} onclick={() => doDelete(true)}>
					Delete Permanently
				</Button>
				<AlertDialog.Action disabled={submitting} onclick={() => doDelete(false)}>
					Move to Trash
				</AlertDialog.Action>
			{:else}
				<AlertDialog.Action disabled={submitting} onclick={() => doDelete(true)}>
					Delete
				</AlertDialog.Action>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
