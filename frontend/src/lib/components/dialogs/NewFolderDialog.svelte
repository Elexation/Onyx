<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { mkdir } from "$lib/api/files.js";
	import { toast } from "svelte-sonner";

	let {
		open = $bindable(false),
		parentPath,
		onsuccess,
	}: {
		open: boolean;
		parentPath: string;
		onsuccess: () => void;
	} = $props();

	let folderName = $state("");
	let submitting = $state(false);
	let inputRef = $state<HTMLInputElement | null>(null);

	$effect(() => {
		if (open) {
			folderName = "";
			requestAnimationFrame(() => inputRef?.focus());
		}
	});

	async function submit() {
		const trimmed = folderName.trim();
		if (!trimmed) return;
		const fullPath = parentPath ? `${parentPath}/${trimmed}` : trimmed;
		submitting = true;
		try {
			await mkdir(fullPath);
			toast.success(`Created folder "${trimmed}"`);
			open = false;
			onsuccess();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to create folder");
		} finally {
			submitting = false;
		}
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
			<Dialog.Title>New Folder</Dialog.Title>
		</Dialog.Header>
		<Input
			bind:value={folderName}
			bind:ref={inputRef}
			onkeydown={handleKeydown}
			disabled={submitting}
			placeholder="Folder name"
		/>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
			<Button onclick={submit} disabled={submitting || !folderName.trim()}>Create</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
