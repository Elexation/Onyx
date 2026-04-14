<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { deleteFiles } from "$lib/api/files.js";
	import { toast } from "svelte-sonner";

	let {
		open = $bindable(false),
		paths,
		onsuccess,
	}: {
		open: boolean;
		paths: string[];
		onsuccess: () => void;
	} = $props();

	let submitting = $state(false);

	const message = $derived(
		paths.length === 1
			? `Delete "${paths[0].split("/").pop()}"?`
			: `Delete ${paths.length} items?`
	);

	async function confirm() {
		submitting = true;
		try {
			const res = await deleteFiles(paths);
			const failed = res.results.filter((r) => !r.success);
			if (failed.length === 0) {
				toast.success(paths.length === 1 ? "Deleted" : `Deleted ${paths.length} items`);
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
			<AlertDialog.Title>{message}</AlertDialog.Title>
			<AlertDialog.Description>This action cannot be undone.</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={confirm} disabled={submitting}>Delete</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
