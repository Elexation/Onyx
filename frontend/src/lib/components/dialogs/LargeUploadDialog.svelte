<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import TriangleAlertIcon from "@lucide/svelte/icons/triangle-alert";
	import { formatFileSize } from "$lib/utils/format";
	import DialogCallout from "./DialogCallout.svelte";

	let {
		fileCount,
		totalBytes,
		onconfirm,
		oncancel,
	}: {
		fileCount: number;
		totalBytes: number;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<AlertDialog.Root open={true}>
	<AlertDialog.Content escapeKeydownBehavior="ignore" interactOutsideBehavior="ignore">
		<AlertDialog.Header>
			<AlertDialog.Title>Upload {fileCount.toLocaleString()} files?</AlertDialog.Title>
			<AlertDialog.Description>
				This folder contains a large number of files. Uploading them all at once may
				take a while and use significant memory in the browser.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<DialogCallout icon={TriangleAlertIcon}>
			<div class="font-mono text-xs text-muted-foreground">
				{fileCount.toLocaleString()} files · {formatFileSize(totalBytes)}
			</div>
		</DialogCallout>

		<AlertDialog.Footer>
			<Button variant="outline" size="sm" onclick={oncancel}>Cancel</Button>
			<Button size="sm" onclick={onconfirm}>Upload anyway</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
