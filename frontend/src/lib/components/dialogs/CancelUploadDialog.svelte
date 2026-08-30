<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import TriangleAlertIcon from "@lucide/svelte/icons/triangle-alert";
	import DialogCallout from "./DialogCallout.svelte";

	let {
		onconfirm,
		oncancel,
	}: {
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<AlertDialog.Root open={true}>
	<AlertDialog.Content escapeKeydownBehavior="ignore" interactOutsideBehavior="ignore">
		<AlertDialog.Header>
			<AlertDialog.Title>Stop upload?</AlertDialog.Title>
			<AlertDialog.Description>
				Files already uploaded are kept. The remaining files won't be uploaded.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<DialogCallout icon={TriangleAlertIcon}>
			<div class="text-xs text-muted-foreground">
				You can re-drop the folder later to finish the remaining files.
			</div>
		</DialogCallout>

		<AlertDialog.Footer>
			<Button variant="outline" size="sm" onclick={oncancel}>Keep going</Button>
			<Button variant="destructive" size="sm" onclick={onconfirm}>Stop upload</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
