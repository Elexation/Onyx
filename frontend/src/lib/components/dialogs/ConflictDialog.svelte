<script lang="ts" module>
	// size is null when it carries no meaning, e.g. the raw inode size of a
	// folder the server did not walk. Rendered as "Folder" instead.
	export type ConflictSide = { size: number | null; modTime: number; isDir: boolean };
	export type ConflictPair = {
		path: string;
		existing: ConflictSide;
		incoming: ConflictSide;
	};
</script>

<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import FileIcon from "$lib/components/FileIcon.svelte";
	import { formatFileSize, formatDate } from "$lib/utils/format";

	type Resolution = "replace" | "keepBoth" | "skip";

	let {
		conflicts,
		kind = "upload",
		onresolve,
	}: {
		conflicts: ConflictPair[];
		kind?: "upload" | "restore";
		onresolve: (resolutions: Record<string, Resolution>) => void;
	} = $props();

	const incomingLabel = $derived(kind === "restore" ? "Restoring" : "Uploading");
	const description = $derived(
		kind === "restore"
			? "Choose how to handle this restore."
			: "Choose how to handle this upload.",
	);

	let index = $state(0);
	let applyToAll = $state(false);
	const resolutions: Record<string, Resolution> = {};

	const current = $derived(conflicts[index]);
	const currentName = $derived(current?.path.split("/").pop() ?? "");
	const total = $derived(conflicts.length);
	const remaining = $derived(total - index);
	const hasMany = $derived(total > 1);

	const mismatched = (p: ConflictPair) => p.existing.isDir !== p.incoming.isDir;

	// Replacing a folder with a file is a hard 422 at finalize, so don't offer
	// it. Restores may replace either way: the occupant goes to trash first.
	const blockedByType = $derived.by(() => {
		if (kind === "restore") return false;
		if (applyToAll) return conflicts.slice(index).some(mismatched);
		return !!current && mismatched(current);
	});

	function choose(action: Resolution) {
		if (applyToAll) {
			for (let i = index; i < conflicts.length; i++) {
				resolutions[conflicts[i].path] = action;
			}
			onresolve({ ...resolutions });
			return;
		}
		resolutions[current.path] = action;
		if (index + 1 >= total) {
			onresolve({ ...resolutions });
		} else {
			index++;
		}
	}
</script>

<!-- Declared outside AlertDialog.Content: a snippet placed directly inside a
	component is passed to it as a prop instead of staying local. -->
{#snippet pane(label: string, side: ConflictSide)}
	<div class="rounded-lg border border-border bg-muted p-3">
		<div class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
			{label}
		</div>
		<div class="flex items-center gap-2.5">
			<FileIcon name={currentName} isDir={side.isDir} class="size-7 shrink-0" />
			<div class="min-w-0">
				<div class="truncate text-meta font-medium">{currentName}{side.isDir ? "/" : ""}</div>
				<div class="font-mono text-xs text-muted-foreground">
					{side.size === null ? "Folder" : formatFileSize(side.size)} · {formatDate(side.modTime)}
				</div>
			</div>
		</div>
	</div>
{/snippet}

<AlertDialog.Root open={true}>
	<AlertDialog.Content
		escapeKeydownBehavior="ignore"
		interactOutsideBehavior="ignore"
	>
		<AlertDialog.Header>
			<AlertDialog.Title>An item with this name already exists</AlertDialog.Title>
			<AlertDialog.Description>
				{#if hasMany}
					Conflict {index + 1} of {total}. Choose how to resolve, or apply to all.
				{:else}
					{description}
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if current}
			<div class="mt-1 grid grid-cols-2 gap-2.5">
				{@render pane("Existing", current.existing)}
				{@render pane(incomingLabel, current.incoming)}
			</div>
		{/if}

		{#if blockedByType}
			<p class="text-xs text-muted-foreground">
				{applyToAll
					? "Replace isn't available: some remaining conflicts are folders."
					: "Replace isn't available: a folder with this name already exists."}
			</p>
		{/if}

		{#if hasMany}
			<label
				class="mt-3 flex cursor-pointer items-center gap-2 text-meta text-muted-foreground"
			>
				<Checkbox bind:checked={applyToAll} />
				Apply to all {remaining} conflicts
			</label>
		{/if}

		<AlertDialog.Footer>
			<Button variant="outline" size="sm" onclick={() => choose("skip")}>Skip</Button>
			<Button variant="outline" size="sm" onclick={() => choose("keepBoth")}>Keep both</Button>
			{#if !blockedByType}
				<Button size="sm" onclick={() => choose("replace")}>Replace</Button>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
