<script lang="ts">
	import type { FileInfo } from "$lib/types";
	import { formatFileSize, formatDate } from "$lib/utils/format.js";
	import { preferences } from "$lib/stores/preferences.svelte.js";
	import { selection } from "$lib/stores/selection.svelte.js";
	import { clipboard } from "$lib/stores/clipboard.svelte.js";
	import { ArrowUp, ArrowDown, ChevronRight, MoreVertical } from "lucide-svelte";
	import type { SortField } from "$lib/stores/preferences.svelte.js";
	import FileIcon from "./FileIcon.svelte";
	import ThumbnailImage from "./ThumbnailImage.svelte";
	import FileContextMenu from "./FileContextMenu.svelte";
	import FileDropdownMenu from "./FileDropdownMenu.svelte";
	import VirtualList from "./VirtualList.svelte";
	import { longpress } from "$lib/actions/longpress.js";
	import { draggable } from "$lib/actions/draggable.js";
	import { droppable } from "$lib/actions/droppable.js";
	import { setupMarquee } from "$lib/actions/marquee.js";

	let {
		items,
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
		items: FileInfo[];
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

	const columns: { label: string; field: SortField; align: "left" | "right" }[] = [
		{ label: "Name", field: "name", align: "left" },
		{ label: "Size", field: "size", align: "right" },
		{ label: "Modified", field: "modified", align: "right" },
	];

	function handleSort(field: SortField) {
		if (preferences.sortField === field) {
			preferences.sortDir = preferences.sortDir === "asc" ? "desc" : "asc";
		} else {
			preferences.sortField = field;
			preferences.sortDir = "asc";
		}
	}

	function handleRowClick(e: MouseEvent, item: FileInfo) {
		e.stopPropagation();
		if (e.shiftKey) {
			e.preventDefault();
			selection.selectRange(item.path, items.filter((i) => i.name !== "..").map((i) => i.path));
		} else if (e.ctrlKey || e.metaKey) {
			e.preventDefault();
			selection.toggle(item.path);
		} else {
			selection.select(item.path);
		}
	}

	function handleRowKeydown(e: KeyboardEvent, item: FileInfo) {
		if (e.key === "Enter") {
			e.preventDefault();
			onopen(item);
		} else if (e.key === " ") {
			e.preventDefault();
			selection.toggle(item.path);
		}
	}

	function getContextPaths(item: FileInfo): string[] {
		return selection.has(item.path) && selection.count > 1 ? [...selection.items] : [item.path];
	}

	let scrollEl = $state<HTMLDivElement | null>(null);
	let headerEl = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (!scrollEl) return;
		return setupMarquee(scrollEl, {
			getLayout: () => ({
				mode: "list",
				rowHeight: 48,
				headerOffset: headerEl?.offsetHeight ?? 0,
			}),
			getItems: () => items,
		});
	});

	const GRID_COLS =
		"grid-cols-[1fr_auto] md:grid-cols-[minmax(0,1fr)_100px_140px_40px] md:gap-3";
</script>

{#if items.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
		<p class="text-sm">This folder is empty</p>
	</div>
{:else}
	<div
		bind:this={scrollEl}
		class="relative min-h-0 flex-1 overflow-auto px-[14px] pb-[14px]"
	>
		<div class="overflow-hidden rounded-xl border border-border bg-card">
			<!-- Header: desktop only -->
			<div
				bind:this={headerEl}
				class="hidden border-b border-border font-mono text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid {GRID_COLS} md:px-[14px] md:py-2.5"
			>
				{#each columns as col}
					<button
						class="inline-flex items-center gap-1 transition-colors hover:text-foreground {col.align ===
						'right'
							? 'justify-end text-right'
							: 'text-left'}"
						onclick={() => handleSort(col.field)}
					>
						{col.label}
						{#if preferences.sortField === col.field}
							{#if preferences.sortDir === "asc"}
								<ArrowUp class="size-3" />
							{:else}
								<ArrowDown class="size-3" />
							{/if}
						{/if}
					</button>
				{/each}
				<div></div>
			</div>

			<VirtualList {items} estimateSize={() => 48} externalScrollEl={scrollEl}>
				{#snippet row({ item, style })}
					{@const file = item as FileInfo}
					{#if file.name === ".."}
						<div
							class="grid cursor-pointer items-center border-b border-border text-muted-foreground transition-colors select-none last:border-b-0 hover:bg-muted {GRID_COLS} px-[14px] py-3.5 md:py-[11px]"
							{style}
							ondblclick={(e) => {
								e.stopPropagation();
								onopen(file);
							}}
							onkeydown={(e) => {
								if (e.key === "Enter" || e.key === " ") {
									e.preventDefault();
									onopen(file);
								}
							}}
							use:droppable={{ path: file.path, ondrop }}
							tabindex={0}
							role="row"
						>
							<div class="flex min-w-0 items-center gap-3 md:gap-3">
								<FileIcon
									isDir={true}
									class="size-7 opacity-60 md:size-6"
									strokeWidth={1.4}
								/>
								<span class="truncate text-[15px] md:text-base">..</span>
							</div>
							<div class="md:hidden"></div>
							<div class="hidden md:block"></div>
							<div class="hidden md:block"></div>
							<div class="hidden md:block"></div>
						</div>
					{:else}
						{@const isSelected = selection.has(file.path)}
						{@const isCut = clipboard.isCut(file.path)}
						{@const lastDot = file.name.lastIndexOf(".")}
						{@const ext = !file.isDir && lastDot > 0 ? file.name.slice(lastDot + 1, lastDot + 5).toUpperCase() : null}
						<FileContextMenu
							item={file}
							onopen={() => onopen(file)}
							onrename={() => onrename(file)}
							ondelete={() => ondelete(getContextPaths(file))}
							{onpaste}
							onmoveto={() => onmoveto(getContextPaths(file))}
							oncopyto={() => oncopyto(getContextPaths(file))}
							onversions={() => onversions(file)}
							onshare={() => onshare(file)}
						>
							{#snippet children(triggerProps)}
								<div
									{...triggerProps}
									class="grid cursor-pointer items-center border-b border-border transition-colors select-none last:border-b-0 {GRID_COLS} px-[14px] py-3.5 md:py-[11px]
										{isSelected ? 'bg-accent-brand-dim' : 'hover:bg-muted'}
										{isCut ? 'opacity-50' : ''}"
									{style}
									onclick={(e) => handleRowClick(e, file)}
									ondblclick={(e) => {
										e.stopPropagation();
										onopen(file);
									}}
									onkeydown={(e) => handleRowKeydown(e, file)}
									use:longpress={() => selection.toggle(file.path)}
									use:draggable={{ path: file.path, isDir: file.isDir }}
									use:droppable={{ path: file.path, ondrop, enabled: file.isDir }}
									tabindex={0}
									role="row"
								>
									<div class="flex min-w-0 items-center gap-3 md:gap-3">
										{#if !file.isDir && (file.mimeType?.startsWith("image/") || file.mimeType?.startsWith("video/"))}
											<ThumbnailImage
												path={file.path}
												size="small"
												class="flex size-7 shrink-0 items-center justify-center overflow-hidden rounded md:size-6"
											>
												{#snippet children()}
													<FileIcon
														mimeType={file.mimeType}
														isDir={false}
														class="size-4 text-muted-foreground"
														strokeWidth={1.4}
													/>
												{/snippet}
											</ThumbnailImage>
										{:else}
											<FileIcon
												mimeType={file.mimeType}
												isDir={file.isDir}
												class="size-7 shrink-0 md:size-6 {file.isDir ? 'text-accent-brand' : 'text-muted-foreground'}"
												strokeWidth={1.4}
											/>
										{/if}
										<span class="min-w-0 flex-1 truncate text-[15px] font-medium md:text-base">
											{file.name}
										</span>
										{#if ext}
											<span
												class="hidden shrink-0 rounded-[5px] bg-muted px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] text-muted-foreground md:inline-flex"
											>
												{ext}
											</span>
										{/if}
									</div>
									<div
										class="flex shrink-0 items-center gap-2 font-mono text-xs text-muted-foreground md:hidden"
									>
										{file.isDir ? "—" : formatFileSize(file.size)}
										{#if file.isDir}
											<ChevronRight class="size-4" strokeWidth={2} />
										{/if}
									</div>
									<div
										class="hidden text-right font-mono text-[13px] text-muted-foreground md:block"
									>
										{file.isDir ? "—" : formatFileSize(file.size)}
									</div>
									<div
										class="hidden text-right font-mono text-[13px] text-muted-foreground md:block"
									>
										{formatDate(file.modTime)}
									</div>
									<div
										class="hidden items-center justify-end md:flex"
										onclick={(e) => e.stopPropagation()}
										role="presentation"
									>
										<FileDropdownMenu
											item={file}
											onopen={() => onopen(file)}
											onrename={() => onrename(file)}
											ondelete={() => ondelete(getContextPaths(file))}
											{onpaste}
											onmoveto={() => onmoveto(getContextPaths(file))}
											oncopyto={() => oncopyto(getContextPaths(file))}
											onversions={() => onversions(file)}
											onshare={() => onshare(file)}
										>
											{#snippet trigger(triggerProps)}
												<button
													{...triggerProps}
													class="inline-flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
													aria-label="More actions"
												>
													<MoreVertical class="size-4" strokeWidth={2} />
												</button>
											{/snippet}
										</FileDropdownMenu>
									</div>
								</div>
							{/snippet}
						</FileContextMenu>
					{/if}
				{/snippet}
			</VirtualList>
		</div>
	</div>
{/if}

<style>
	:global(.drop-target-active) {
		background-color: var(--accent-brand-dim) !important;
		box-shadow: inset 0 0 0 1px var(--accent-brand);
	}
</style>
