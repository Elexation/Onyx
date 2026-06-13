<script lang="ts">
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import FolderPlusIcon from "@lucide/svelte/icons/folder-plus";
	import FileUpIcon from "@lucide/svelte/icons/file-up";
	import FolderUpIcon from "@lucide/svelte/icons/folder-up";

	let {
		onnewfolder,
		onfiles,
		label = "Create",
	}: {
		onnewfolder: () => void;
		onfiles: (files: File[]) => void;
		label?: string;
	} = $props();

	let fileInput = $state<HTMLInputElement | null>(null);
	let folderInput = $state<HTMLInputElement | null>(null);

	function handleFileChange(e: Event) {
		const input = e.target as HTMLInputElement;
		if (input.files?.length) {
			onfiles(Array.from(input.files));
			input.value = "";
		}
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
			<button
				type="button"
				class="fixed right-5 bottom-5 z-40 inline-flex size-14 items-center justify-center rounded-full bg-accent-brand text-accent-brand-foreground transition-[filter,transform] hover:brightness-110 active:translate-y-px md:hidden"
				aria-label={label}
				{...props}
			>
				<PlusIcon class="size-6" strokeWidth={2.25} />
			</button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content
		align="end"
		side="top"
		sideOffset={8}
		class="w-auto min-w-48"
	>
		<DropdownMenu.Item onclick={onnewfolder}>
			<FolderPlusIcon class="size-4" />
			New folder
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={() => fileInput?.click()}>
			<FileUpIcon class="size-4" />
			File upload
		</DropdownMenu.Item>
		<DropdownMenu.Item onclick={() => folderInput?.click()}>
			<FolderUpIcon class="size-4" />
			Folder upload
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>
