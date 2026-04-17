<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";

	type Resolution = "replace" | "keepBoth" | "skip";

	let {
		conflicts,
		onresolve,
	}: {
		conflicts: string[];
		onresolve: (resolutions: Record<string, Resolution>) => void;
	} = $props();

	function resolveAll(action: Resolution) {
		const resolutions: Record<string, Resolution> = {};
		for (const name of conflicts) {
			resolutions[name] = action;
		}
		onresolve(resolutions);
	}
</script>

<AlertDialog.Root open={true}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{conflicts.length === 1
					? "File already exists"
					: `${conflicts.length} files already exist`}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{#if conflicts.length === 1}
					<span class="font-medium">{conflicts[0]}</span> already exists in this folder.
				{:else}
					The following files already exist:
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if conflicts.length > 1}
			<div class="max-h-40 overflow-y-auto rounded border border-border p-2">
				{#each conflicts as name}
					<div class="truncate text-xs text-muted-foreground">{name}</div>
				{/each}
			</div>
		{/if}

		<AlertDialog.Footer>
			<Button variant="outline" size="sm" onclick={() => resolveAll("skip")}>
				Skip
			</Button>
			<Button variant="outline" size="sm" onclick={() => resolveAll("keepBoth")}>
				Keep Both
			</Button>
			<Button size="sm" onclick={() => resolveAll("replace")}>
				Replace
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
