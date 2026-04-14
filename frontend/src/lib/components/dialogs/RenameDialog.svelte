<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
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

	async function submit() {
		const trimmed = newName.trim();
		if (!trimmed || trimmed === name) {
			open = false;
			return;
		}
		submitting = true;
		try {
			await rename(path, trimmed);
			toast.success(`Renamed to "${trimmed}"`);
			open = false;
			onsuccess();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Rename failed");
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
			<Button onclick={submit} disabled={submitting || !newName.trim()}>Rename</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
