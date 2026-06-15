<script lang="ts" module>
	type CacheState = "loaded" | "failed";
	type CacheEntry = { state: CacheState; url?: string };
	const CACHE_LIMIT = 200;
	const cache = new Map<string, CacheEntry>();

	function cacheKey(path: string, size: string) {
		return size + "\0" + path;
	}

	// Evicts the oldest entry when the cache exceeds CACHE_LIMIT, revoking its
	// blob URL so the underlying Blob can be GC'd.
	function cacheSet(key: string, entry: CacheEntry) {
		if (cache.has(key)) cache.delete(key);
		cache.set(key, entry);
		while (cache.size > CACHE_LIMIT) {
			const oldestKey = cache.keys().next().value;
			if (oldestKey === undefined) break;
			const oldest = cache.get(oldestKey);
			cache.delete(oldestKey);
			if (oldest?.url) URL.revokeObjectURL(oldest.url);
		}
	}
</script>

<script lang="ts">
	import { onDestroy } from "svelte";
	import type { Snippet } from "svelte";
	import { encodeFilePath } from "$lib/utils";

	let {
		path,
		size = "medium",
		class: className = "",
		children,
	}: {
		path: string;
		size?: "small" | "medium" | "large";
		class?: string;
		children: Snippet;
	} = $props();

	type State = "idle" | "loading" | "loaded" | "failed";
	let loadState: State = $state("idle");
	let url: string | null = $state(null);
	let el: HTMLDivElement | null = $state(null);
	let ownedUrl: string | null = null;

	const key = $derived(cacheKey(path, size));

	$effect(() => {
		const cached = cache.get(key);
		// Revoke any previously-owned blob URL that no longer matches the current
		// cache entry before resetting state for the new key.
		if (ownedUrl && cached?.url !== ownedUrl) {
			URL.revokeObjectURL(ownedUrl);
			ownedUrl = null;
		}
		if (cached) {
			loadState = cached.state;
			url = cached.url ?? null;
			return;
		}
		loadState = "idle";
		url = null;
	});

	$effect(() => {
		if (!el || loadState !== "idle") return;
		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						observer.disconnect();
						load();
						break;
					}
				}
			},
			{ rootMargin: "200px" },
		);
		observer.observe(el);
		return () => observer.disconnect();
	});

	async function load() {
		loadState = "loading";
		const maxAttempts = 3;
		for (let attempt = 1; attempt <= maxAttempts; attempt++) {
			try {
				const res = await fetch(
					`/api/thumbs${encodeFilePath(path)}?size=${size}`,
					{ credentials: "same-origin" },
				);
				if (res.status === 200) {
					const blob = await res.blob();
					const objectUrl = URL.createObjectURL(blob);
					ownedUrl = objectUrl;
					url = objectUrl;
					loadState = "loaded";
					cacheSet(key, { state: "loaded", url: objectUrl });
					return;
				}
				if (res.status === 202 && attempt < maxAttempts) {
					await new Promise((r) => setTimeout(r, 2000));
					continue;
				}
				fail();
				return;
			} catch {
				fail();
				return;
			}
		}
		fail();
	}

	function fail() {
		loadState = "failed";
		url = null;
		cacheSet(key, { state: "failed" });
	}

	onDestroy(() => {
		if (ownedUrl && cache.get(key)?.url !== ownedUrl) {
			URL.revokeObjectURL(ownedUrl);
		}
	});
</script>

<div bind:this={el} class={className}>
	{#if loadState === "loaded" && url}
		<img src={url} alt="" class="h-full w-full rounded object-cover" loading="lazy" />
	{:else}
		{@render children()}
	{/if}
</div>
