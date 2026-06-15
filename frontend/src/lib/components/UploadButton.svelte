<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import UploadIcon from "@lucide/svelte/icons/upload";
	import FileIcon from "@lucide/svelte/icons/file";
	import FolderIcon from "@lucide/svelte/icons/folder";

	let {
		onfiles,
	}: {
		onfiles: (files: File[]) => void;
	} = $props();

	let fileInput: HTMLInputElement;
	let folderInput: HTMLInputElement;

	function handleFileChange(e: Event) {
		const input = e.target as HTMLInputElement;
		if (input.files?.length) {
			onfiles(Array.from(input.files));
			input.value = "";
		}
	}

	export function openFilePicker() {
		fileInput?.click();
	}
</script>

<input
	bind:this={fileInput}
	type="file"
	multiple
	class="hidden"
	onchange={handleFileChange}
/>
<input
	bind:this={folderInput}
	type="file"
	webkitdirectory
	class="hidden"
	onchange={handleFileChange}
/>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button variant="default" size="sm" {...props}>
				<UploadIcon class="size-[15px]" strokeWidth={2} />
				Upload
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="start">
		<DropdownMenu.Item onclick={() => fileInput.click()}>
			<FileIcon class="size-4" />
			Files
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={() => folderInput.click()}>
			<FolderIcon class="size-4" />
			Folder
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>
