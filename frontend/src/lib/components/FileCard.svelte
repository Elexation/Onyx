<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { formatFileSize } from "$lib/utils/format.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import FileIcon from "./FileIcon.svelte";
	import ThumbnailImage from "./ThumbnailImage.svelte";
	import FileContextMenu from "./FileContextMenu.svelte";
	import { Link2 } from "lucide-svelte";
	import { sharedPaths } from "$lib/stores/sharedPaths.svelte.js";
	import { longpress } from "$lib/actions/longpress.js";
	import { draggable } from "$lib/actions/draggable.js";
	import { droppable } from "$lib/actions/droppable.js";

	let {
		item,
		allPaths,
		onopen,
		onrename,
		ondelete,
		onpaste,
		onmoveto,
		oncopyto,
		onversions,
		onshare,
		ondrop,
	}: {
		item: FileInfo;
		allPaths: string[];
		onopen: (item: FileInfo) => void;
		onrename: (item: FileInfo) => void;
		ondelete: (paths: string[]) => void;
		onpaste: () => void;
		onmoveto: (paths: string[]) => void;
		oncopyto: (paths: string[]) => void;
		onversions: (item: FileInfo) => void;
		onshare: (item: FileInfo) => void;
		ondrop: (paths: string[], destination: string) => void;
	} = $props();

	const isSelected = $derived(selection.has(item.path));
	const isCut = $derived(clipboard.isCut(item.path));
	const hasThumbnail = $derived(
		!item.isDir &&
			(item.mimeType?.startsWith("image/") || item.mimeType?.startsWith("video/")),
	);
	const lastDot = $derived(item.name.lastIndexOf("."));
	const ext = $derived(
		!item.isDir && lastDot > 0 ? item.name.slice(lastDot + 1, lastDot + 5).toUpperCase() : null,
	);

	function handleClick(e: MouseEvent) {
		e.stopPropagation();
		if (e.shiftKey) {
			e.preventDefault();
			selection.selectRange(item.path, allPaths);
		} else if (e.ctrlKey || e.metaKey) {
			e.preventDefault();
			selection.toggle(item.path);
		} else {
			selection.select(item.path);
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "Enter") {
			e.preventDefault();
			onopen(item);
		} else if (e.key === " ") {
			e.preventDefault();
			selection.toggle(item.path);
		}
	}

	function getContextPaths(): string[] {
		return selection.has(item.path) && selection.count > 1 ? [...selection.items] : [item.path];
	}
</script>

{#if item.name === ".."}
	<div
		class="flex cursor-pointer flex-col items-center gap-2 rounded-xl border border-border bg-card p-2.5 text-muted-foreground transition-colors select-none hover:border-border-2"
		ondblclick={(e) => {
			e.stopPropagation();
			onopen(item);
		}}
		onkeydown={(e) => {
			if (e.key === "Enter" || e.key === " ") {
				e.preventDefault();
				onopen(item);
			}
		}}
		use:droppable={{ path: item.path, ondrop }}
		tabindex={0}
		role="gridcell"
	>
		<div class="mx-auto flex h-24 w-24 items-center justify-center">
			<FileIcon isDir={true} class="size-12 opacity-50" strokeWidth={1.2} />
		</div>
		<span class="w-full truncate text-center text-sm font-medium">..</span>
	</div>
{:else}
	<FileContextMenu
		{item}
		onopen={() => onopen(item)}
		onrename={() => onrename(item)}
		ondelete={() => ondelete(getContextPaths())}
		{onpaste}
		onmoveto={() => onmoveto(getContextPaths())}
		oncopyto={() => oncopyto(getContextPaths())}
		onversions={() => onversions(item)}
		onshare={() => onshare(item)}
	>
		{#snippet children(triggerProps)}
			<div
				{...triggerProps}
				class="relative flex cursor-pointer flex-col items-center gap-2 rounded-xl border p-2.5 transition-colors select-none
					{isSelected
					? 'border-accent-brand bg-accent-brand-dim'
					: 'border-border bg-card hover:border-border-2'}
					{isCut ? 'opacity-50' : ''}"
				onclick={handleClick}
				ondblclick={(e) => {
					e.stopPropagation();
					onopen(item);
				}}
				onkeydown={handleKeydown}
				use:longpress={() => selection.toggle(item.path)}
				use:draggable={{ path: item.path, isDir: item.isDir }}
				use:droppable={{ path: item.path, ondrop, enabled: item.isDir }}
				tabindex={0}
				role="gridcell"
			>
				<div class="relative mx-auto w-24">
					<div
						class="relative flex h-24 w-24 items-center justify-center overflow-hidden rounded-lg"
					>
						{#if hasThumbnail}
							<ThumbnailImage
								path={item.path}
								size="large"
								class="flex h-full w-full items-center justify-center"
							>
								{#snippet children()}
									<FileIcon
										mimeType={item.mimeType}
										name={item.name}
										isDir={false}
										class="size-10"
										strokeWidth={1.2}
									/>
								{/snippet}
							</ThumbnailImage>
						{:else}
							<FileIcon
								mimeType={item.mimeType}
								name={item.name}
								isDir={item.isDir}
								class={item.isDir ? "size-12 text-accent-brand" : "size-10 text-muted-foreground"}
								strokeWidth={1.2}
							/>
						{/if}
					</div>
					{#if !hasThumbnail && ext}
						<span
							class="absolute -right-[2px] -bottom-[2px] rounded border border-border-2 bg-background px-[5px] py-[1px] font-mono text-[9px] font-semibold tracking-wider text-muted-foreground uppercase"
						>
							{ext}
						</span>
					{/if}
				</div>
				<span class="w-full truncate text-center text-sm font-medium text-foreground">
					{item.name}
				</span>
				<span class="w-full truncate text-center text-[11px] tabular-nums text-muted-foreground">
					{item.isDir ? `${item.itemCount ?? 0} items` : formatFileSize(item.size)}
				</span>
				{#if sharedPaths.has(item.path)}
					<span
						class="absolute top-2 right-2 flex size-5 items-center justify-center rounded-full border border-border-2 bg-background text-accent-brand"
						aria-label="Shared"
					>
						<Link2 class="size-[11px]" strokeWidth={2.5} />
					</span>
				{/if}
			</div>
		{/snippet}
	</FileContextMenu>
{/if}
