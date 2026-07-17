<script lang="ts" module>
	type CacheState = "loaded" | "failed";
	type CacheEntry = { state: CacheState; blob?: Blob };
	const CACHE_LIMIT = 200;
	const cache = new Map<string, CacheEntry>();

	function cacheKey(path: string, size: string) {
		return size + "\0" + path;
	}

	function cacheSet(key: string, entry: CacheEntry) {
		if (cache.has(key)) cache.delete(key);
		cache.set(key, entry);
		while (cache.size > CACHE_LIMIT) {
			const oldestKey = cache.keys().next().value;
			if (oldestKey === undefined) break;
			cache.delete(oldestKey);
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
	let imgRetried = false;

	const key = $derived(cacheKey(path, size));

	function clearOwnedUrl() {
		if (ownedUrl) {
			URL.revokeObjectURL(ownedUrl);
			ownedUrl = null;
		}
	}

	$effect(() => {
		// Re-run when key changes (path/size prop change on a live instance).
		// Revoke the previous instance-owned URL before adopting state for the new key.
		clearOwnedUrl();
		imgRetried = false;
		const cached = cache.get(key);
		if (cached?.state === "loaded" && cached.blob) {
			const objectUrl = URL.createObjectURL(cached.blob);
			ownedUrl = objectUrl;
			url = objectUrl;
			loadState = "loaded";
			return;
		}
		if (cached?.state === "failed") {
			loadState = "failed";
			url = null;
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
					clearOwnedUrl();
					ownedUrl = objectUrl;
					url = objectUrl;
					loadState = "loaded";
					cacheSet(key, { state: "loaded", blob });
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
		clearOwnedUrl();
		loadState = "failed";
		url = null;
		cacheSet(key, { state: "failed" });
	}

	function handleImgError() {
		// The created URL didn't resolve in the <img>. Drop the cache entry and
		// retry once via the IntersectionObserver path; on a second failure, fall
		// through to the children fallback (FileIcon) — never the browser default.
		if (imgRetried) {
			fail();
			return;
		}
		imgRetried = true;
		cache.delete(key);
		clearOwnedUrl();
		loadState = "idle";
		url = null;
	}

	onDestroy(() => {
		clearOwnedUrl();
	});
</script>

<div bind:this={el} class={className}>
	{#if loadState === "loaded" && url}
		<img
			src={url}
			alt=""
			class="h-full w-full rounded object-cover"
			loading="lazy"
			onerror={handleImgError}
		/>
	{:else}
		{@render children()}
	{/if}
</div>
