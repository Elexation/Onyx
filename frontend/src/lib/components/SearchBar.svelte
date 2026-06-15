<script lang="ts">
	import { goto } from "$app/navigation";
	import { search, type SearchResponse } from "$lib/api/search.js";
	import type { SearchResult } from "$lib/types";
	import FileIcon from "./FileIcon.svelte";
	import { Search } from "lucide-svelte";

	let {
		autoFocusKey = 0,
		onescape,
	}: {
		autoFocusKey?: number;
		onescape?: () => void;
	} = $props();

	let query = $state("");
	let results = $state<SearchResult[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let open = $state(false);
	let activeIndex = $state(-1);
	let containerEl = $state<HTMLDivElement | null>(null);
	let inputEl = $state<HTMLInputElement | null>(null);
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;
	let searchSeq = 0;

	$effect(() => {
		const q = query.trim();
		clearTimeout(debounceTimer);

		if (q.length < 2) {
			searchSeq++;
			results = [];
			total = 0;
			open = false;
			return;
		}

		debounceTimer = setTimeout(async () => {
			const seq = ++searchSeq;
			loading = true;
			try {
				const res: SearchResponse = await search(q);
				if (seq !== searchSeq) return;
				results = res.results;
				total = res.total;
				activeIndex = -1;
				open = true;
			} catch {
				if (seq !== searchSeq) return;
				results = [];
				total = 0;
			} finally {
				if (seq === searchSeq) loading = false;
			}
		}, 200);

		return () => {
			clearTimeout(debounceTimer);
			searchSeq++;
		};
	});

	// Imperative focus trigger: parent increments autoFocusKey to request focus.
	$effect(() => {
		autoFocusKey;
		if (autoFocusKey > 0) {
			// Defer to next microtask so any hidden→visible transition settles first.
			queueMicrotask(() => inputEl?.focus());
		}
	});

	function navigateTo(result: SearchResult) {
		open = false;
		query = "";
		results = [];
		inputEl?.blur();

		if (result.isDir) {
			goto(`/files${result.path}`);
		} else {
			const lastSlash = result.path.lastIndexOf("/");
			const parentDir = lastSlash > 0 ? result.path.substring(0, lastSlash) : "";
			goto(`/files${parentDir}`);
		}
	}

	function onkeydown(e: KeyboardEvent) {
		if (!open || results.length === 0) {
			if (e.key === "Escape") {
				query = "";
				inputEl?.blur();
				onescape?.();
			}
			return;
		}

		switch (e.key) {
			case "ArrowDown":
				e.preventDefault();
				activeIndex = Math.min(activeIndex + 1, results.length - 1);
				break;
			case "ArrowUp":
				e.preventDefault();
				activeIndex = Math.max(activeIndex - 1, -1);
				break;
			case "Enter":
				e.preventDefault();
				if (activeIndex >= 0) {
					navigateTo(results[activeIndex]);
				} else if (results.length > 0) {
					navigateTo(results[0]);
				}
				break;
			case "Escape":
				e.preventDefault();
				open = false;
				query = "";
				inputEl?.blur();
				onescape?.();
				break;
		}
	}

	function onclickOutside(e: MouseEvent) {
		if (containerEl && !containerEl.contains(e.target as Node)) {
			open = false;
		}
	}

	function highlightSegments(name: string, q: string): { text: string; match: boolean }[] {
		const tokens = q.trim().toLowerCase().split(/\s+/).filter(Boolean);
		if (tokens.length === 0) return [{ text: name, match: false }];

		const lower = name.toLowerCase();
		const marked = new Uint8Array(name.length);

		for (const token of tokens) {
			let pos = 0;
			while (true) {
				const idx = lower.indexOf(token, pos);
				if (idx === -1) break;
				for (let i = idx; i < idx + token.length && i < name.length; i++) {
					marked[i] = 1;
				}
				pos = idx + 1;
			}
		}

		const segments: { text: string; match: boolean }[] = [];
		let i = 0;
		while (i < name.length) {
			const isMatch = marked[i] === 1;
			let j = i + 1;
			while (j < name.length && (marked[j] === 1) === isMatch) j++;
			segments.push({ text: name.slice(i, j), match: isMatch });
			i = j;
		}
		return segments;
	}
</script>

<svelte:window onclick={onclickOutside} />

<div class="relative w-full" bind:this={containerEl}>
	<div
		class="flex h-9 items-center gap-2 rounded-lg border border-border bg-background px-3 transition-colors focus-within:border-ring"
	>
		<Search class="size-4 shrink-0 text-muted-foreground" strokeWidth={2} />
		<input
			bind:this={inputEl}
			bind:value={query}
			{onkeydown}
			onfocus={() => {
				if (results.length > 0 && query.trim().length >= 2) open = true;
			}}
			type="text"
			placeholder="Search files, folders, contents…"
			class="flex-1 bg-transparent text-sm text-foreground placeholder:text-[oklch(0.45_0_0)] focus:outline-none"
		/>
	</div>

	{#if open && results.length > 0}
		<div
			class="absolute top-full left-0 z-50 mt-1.5 w-full overflow-hidden rounded-lg border border-border-2 bg-popover"
		>
			{#each results as result, i}
				<button
					class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-muted {i ===
					activeIndex
						? 'bg-muted'
						: ''}"
					onmouseenter={() => {
						activeIndex = i;
					}}
					onclick={() => navigateTo(result)}
				>
					<FileIcon isDir={result.isDir} class="size-4 shrink-0 text-muted-foreground" />
					<span class="min-w-0 truncate">
						{#each highlightSegments(result.name, query) as seg}
							{#if seg.match}<mark
									class="bg-transparent font-semibold text-foreground">{seg.text}</mark
								>{:else}{seg.text}{/if}
						{/each}
					</span>
					<span
						class="ml-auto max-w-[50%] shrink-0 truncate font-mono text-[11px] text-muted-foreground"
						title={result.path}
					>
						{result.path}
					</span>
				</button>
			{/each}
			{#if total > results.length}
				<div
					class="border-t border-border px-3 py-1.5 font-mono text-[11px] text-muted-foreground"
				>
					{total - results.length} more result{total - results.length === 1 ? "" : "s"}
				</div>
			{/if}
		</div>
	{:else if open && query.trim().length >= 2 && !loading && results.length === 0}
		<div
			class="absolute top-full left-0 z-50 mt-1.5 w-full rounded-lg border border-border-2 bg-popover"
		>
			<div class="px-3 py-4 text-center text-sm text-muted-foreground">No results found</div>
		</div>
	{/if}
</div>
