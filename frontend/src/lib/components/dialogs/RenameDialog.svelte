<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { rename } from "$lib/api/files.js";
	import { toast } from "svelte-sonner";

	let {
		open = $bindable(false),
		path,
		name,
		onsuccess,
	}: {
		open: boolean;
		path: string;
		name: string;
		onsuccess: () => void;
	} = $props();

	let newName = $state("");
	let submitting = $state(false);
	let inputRef = $state<HTMLInputElement | null>(null);
	let confirmExtOpen = $state(false);
	let pendingName = $state("");

	$effect(() => {
		if (open) {
			newName = name;
		}
	});

	$effect(() => {
		if (open && inputRef) {
			requestAnimationFrame(() => {
				if (!inputRef) return;
				inputRef.focus();
				const dot = newName.lastIndexOf(".");
				if (dot > 0) {
					inputRef.setSelectionRange(0, dot);
				} else {
					inputRef.select();
				}
			});
		}
	});

	function extOf(s: string): string {
		const dot = s.lastIndexOf(".");
		return dot > 0 ? s.slice(dot + 1).toLowerCase() : "";
	}

	async function doRename(target: string) {
		submitting = true;
		try {
			await rename(path, target);
			toast.success(`Renamed to "${target}"`);
			open = false;
			onsuccess();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Rename failed");
		} finally {
			submitting = false;
		}
	}

	function submit() {
		const trimmed = newName.trim();
		if (!trimmed || trimmed === name) {
			open = false;
			return;
		}
		if (extOf(trimmed) !== extOf(name)) {
			pendingName = trimmed;
			confirmExtOpen = true;
			return;
		}
		doRename(trimmed);
	}

	function confirmExtChange() {
		confirmExtOpen = false;
		doRename(pendingName);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "Enter") {
			e.preventDefault();
			submit();
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Rename</Dialog.Title>
		</Dialog.Header>
		<Input
			bind:value={newName}
			bind:ref={inputRef}
			onkeydown={handleKeydown}
			disabled={submitting}
		/>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
			<Button onclick={submit} loading={submitting} disabled={!newName.trim()}>
				{submitting ? "Renaming…" : "Rename"}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={confirmExtOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Rename</AlertDialog.Title>
			<AlertDialog.Description>
				If you change a file name extension, the file might become unusable.
				Are you sure you want to change it?
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={submitting}>No</AlertDialog.Cancel>
			<AlertDialog.Action disabled={submitting} onclick={confirmExtChange}>
				Yes
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
