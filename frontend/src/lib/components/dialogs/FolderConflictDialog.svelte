<script lang="ts" module>
	export type FolderConflictPair = {
		name: string;
		existing: { modTime: number };
		incoming: { fileCount: number };
	};
	export type FolderResolution = "merge" | "keepBoth" | "skip";
</script>

<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import { formatDate } from "$lib/utils/format";
	import DialogCallout from "./DialogCallout.svelte";

	let {
		conflicts,
		onresolve,
	}: {
		conflicts: FolderConflictPair[];
		onresolve: (resolutions: Record<string, FolderResolution>) => void;
	} = $props();

	let index = $state(0);
	let applyToAll = $state(false);
	const resolutions: Record<string, FolderResolution> = {};

	const current = $derived(conflicts[index]);
	const total = $derived(conflicts.length);
	const remaining = $derived(total - index);
	const hasMany = $derived(total > 1);

	function choose(action: FolderResolution) {
		if (applyToAll) {
			for (let i = index; i < conflicts.length; i++) {
				resolutions[conflicts[i].name] = action;
			}
			onresolve({ ...resolutions });
			return;
		}
		resolutions[current.name] = action;
		if (index + 1 >= total) {
			onresolve({ ...resolutions });
		} else {
			index++;
		}
	}
</script>

<AlertDialog.Root open={true}>
	<AlertDialog.Content escapeKeydownBehavior="ignore" interactOutsideBehavior="ignore">
		<AlertDialog.Header>
			<AlertDialog.Title>A folder with this name already exists</AlertDialog.Title>
			<AlertDialog.Description>
				{#if hasMany}
					Folder conflict {index + 1} of {total}. Choose how to resolve, or apply to all.
				{:else}
					Choose how to upload into the existing folder.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if current}
			<DialogCallout icon={FolderIcon}>
				<div class="truncate text-meta font-medium">{current.name}/</div>
				<div class="font-mono text-xs text-muted-foreground">
					existing · modified {formatDate(current.existing.modTime)} · uploading {current
						.incoming.fileCount}
					{current.incoming.fileCount === 1 ? "file" : "files"}
				</div>
			</DialogCallout>

			<ul class="mt-1 space-y-1 text-xs text-muted-foreground">
				<li><span class="font-medium text-foreground">Merge</span> — add into the existing folder, overwriting files with the same name (older versions are kept).</li>
				<li><span class="font-medium text-foreground">Keep both</span> — upload as a new folder, e.g. "{current.name} (1)".</li>
				<li><span class="font-medium text-foreground">Skip</span> — don't upload this folder.</li>
			</ul>
		{/if}

		{#if hasMany}
			<label class="mt-3 flex cursor-pointer items-center gap-2 text-meta text-muted-foreground">
				<Checkbox bind:checked={applyToAll} />
				Apply to all {remaining} folder conflicts
			</label>
		{/if}

		<AlertDialog.Footer>
			<Button variant="outline" size="sm" onclick={() => choose("skip")}>Skip</Button>
			<Button variant="outline" size="sm" onclick={() => choose("keepBoth")}>Keep both</Button>
			<Button size="sm" onclick={() => choose("merge")}>Merge</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
