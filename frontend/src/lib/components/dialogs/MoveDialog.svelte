<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { move, copy, listDirectory } from "$lib/api/files.js";
	import type { BatchResult } from "$lib/api/files.js";
	import { toast } from "svelte-sonner";
	import LoadingState from "$lib/components/LoadingState.svelte";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import FolderOpenIcon from "@lucide/svelte/icons/folder-open";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";

	let {
		open = $bindable(false),
		paths,
		mode = "move",
		onsuccess,
	}: {
		open: boolean;
		paths: string[];
		mode: "move" | "copy";
		onsuccess: () => void;
	} = $props();

	let destination = $state("");
	let submitting = $state(false);
	let treeLoading = $state(false);
	let treeError = $state(false);
	let treeContainer = $state<HTMLDivElement | null>(null);

	interface TreeNode {
		name: string;
		path: string;
		hasSubDirs: boolean;
		expanded: boolean;
		loaded: boolean;
		children: TreeNode[];
	}

	let roots = $state<TreeNode[]>([]);

	$effect(() => {
		if (open) {
			destination = "";
			roots = [];
			treeError = false;
			loadChildren("");
		}
	});

	async function loadChildren(parentPath: string) {
		if (parentPath === "") treeLoading = true;
		try {
			const listing = await listDirectory(parentPath, { dirsOnly: true });
			const dirs = listing.items
				.filter((f) => f.isDir)
				.map((f): TreeNode => ({
					name: f.name,
					path: f.path,
					hasSubDirs: f.hasSubDirs ?? false,
					expanded: false,
					loaded: false,
					children: [],
				}));
			if (parentPath === "") {
				roots = dirs;
				treeError = false;
			} else {
				setChildren(roots, parentPath, dirs);
				roots = [...roots];
			}
		} catch {
			if (parentPath === "") treeError = true;
		} finally {
			if (parentPath === "") treeLoading = false;
		}
	}

	function setChildren(nodes: TreeNode[], targetPath: string, children: TreeNode[]) {
		for (const node of nodes) {
			if (node.path === targetPath) {
				node.children = children;
				node.loaded = true;
				if (children.length === 0) node.expanded = false;
				return;
			}
			if (targetPath.startsWith(node.path + "/")) {
				setChildren(node.children, targetPath, children);
			}
		}
	}

	function toggleNode(node: TreeNode) {
		node.expanded = !node.expanded;
		if (node.expanded && !node.loaded) {
			loadChildren(node.path);
		}
		roots = [...roots];
	}

	function selectNode(path: string) {
		destination = path;
	}

	function focusSibling(current: EventTarget | null, direction: 1 | -1) {
		if (!treeContainer || !current) return;
		const items = Array.from(treeContainer.querySelectorAll<HTMLElement>('[role="treeitem"]:not([aria-disabled="true"]), button'));
		const idx = items.indexOf(current as HTMLElement);
		if (idx === -1) return;
		const next = items[idx + direction];
		if (next) next.focus();
	}

	async function submit() {
		submitting = true;
		const fn = mode === "move" ? move : copy;
		const dest = destination || "/";
		try {
			const res = await fn(paths, dest);
			const failed = res.results.filter((r: BatchResult) => !r.success);
			const verb = mode === "move" ? "Moved" : "Copied";
			if (failed.length === 0) {
				toast.success(paths.length === 1 ? `${verb} item` : `${verb} ${paths.length} items`);
			} else {
				toast.error(`${failed.length} item(s) failed`);
			}
			open = false;
			onsuccess();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : `${mode} failed`);
		} finally {
			submitting = false;
		}
	}
</script>

{#snippet treeNodes(nodes: TreeNode[], depth: number)}
	{#each nodes as node}
		{@const isSource = paths.includes(node.path)}
		{@const hasDisclosure = node.loaded ? node.children.length > 0 : node.hasSubDirs}
		<div role="none">
			<div
				class="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-[15px] transition-colors
					{destination === node.path ? 'bg-accent-brand-dim text-foreground' : 'hover:bg-muted'}
					{isSource ? 'pointer-events-none opacity-40' : 'cursor-pointer'}"
				style="padding-left: {depth * 20 + 8}px"
				role="treeitem"
				aria-selected={destination === node.path}
				aria-expanded={node.expanded}
				aria-disabled={isSource}
				tabindex={isSource ? -1 : 0}
				onclick={() => { if (!isSource) { selectNode(node.path); if (hasDisclosure) toggleNode(node); } }}
				onkeydown={(e) => {
					if (isSource) return;
					if (e.key === "Enter" || e.key === " ") {
						e.preventDefault();
						selectNode(node.path);
						if (hasDisclosure) toggleNode(node);
					} else if (e.key === "ArrowRight") {
						e.preventDefault();
						if (hasDisclosure && !node.expanded) toggleNode(node);
					} else if (e.key === "ArrowLeft") {
						e.preventDefault();
						if (hasDisclosure && node.expanded) toggleNode(node);
					} else if (e.key === "ArrowDown") {
						e.preventDefault();
						focusSibling(e.currentTarget, 1);
					} else if (e.key === "ArrowUp") {
						e.preventDefault();
						focusSibling(e.currentTarget, -1);
					}
				}}
			>
				<span class="flex size-4 shrink-0 items-center justify-center">
					{#if hasDisclosure}
						<ChevronRightIcon class="size-3 transition-transform {node.expanded ? 'rotate-90' : ''}" />
					{/if}
				</span>
				{#if node.expanded}
					<FolderOpenIcon class="size-4 shrink-0 text-accent-brand" strokeWidth={2} />
				{:else}
					<FolderIcon class="size-4 shrink-0 text-accent-brand" strokeWidth={2} />
				{/if}
				<span class="truncate">{node.name}</span>
			</div>
			{#if node.expanded && node.children.length > 0}
				{@render treeNodes(node.children, depth + 1)}
			{/if}
		</div>
	{/each}
{/snippet}

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{mode === "move" ? "Move" : "Copy"} to…</Dialog.Title>
		</Dialog.Header>
		<div bind:this={treeContainer} class="min-h-[200px] max-h-64 overflow-y-auto rounded-lg border border-border bg-background p-1.5">
			{#if treeLoading && roots.length === 0}
				<LoadingState compact />
			{:else if treeError && roots.length === 0}
				<div class="flex flex-col items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
					<p>Failed to load directories</p>
					<Button variant="outline" size="sm" onclick={() => loadChildren("")}>Retry</Button>
				</div>
			{:else}
				<button
					class="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-[15px] transition-colors
						{destination === '' ? 'bg-accent-brand-dim text-foreground' : 'hover:bg-muted'}"
					onclick={() => selectNode("")}
					onkeydown={(e) => {
						if (e.key === "ArrowDown") {
							e.preventDefault();
							focusSibling(e.currentTarget, 1);
						}
					}}
				>
					<FolderIcon class="size-4 shrink-0 text-accent-brand" strokeWidth={2} />
					<span class="font-medium">/</span>
				</button>
				{@render treeNodes(roots, 0)}
			{/if}
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
			<Button onclick={submit} loading={submitting}>
				{#if submitting}
					{mode === "move" ? "Moving…" : "Copying…"}
				{:else}
					{mode === "move" ? "Move" : "Copy"} here
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
