<script lang="ts" module>
	type CacheState = "loaded" | "failed";
	type CacheEntry = { state: CacheState; blob?: Blob };
	const CACHE_LIMIT = 200;
	const cache = new Map<string, CacheEntry>();

	function cacheKey(path: string, size: string, modTime?: number) {
		return size + "\0" + path + "\0" + (modTime ?? 0);
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
		modTime,
		class: className = "",
		children,
	}: {
		path: string;
		size?: "small" | "medium" | "large";
		modTime?: number;
		class?: string;
		children: Snippet;
	} = $props();

	type State = "idle" | "loading" | "loaded" | "failed";
	let loadState: State = $state("idle");
	let url: string | null = $state(null);
	let el: HTMLDivElement | null = $state(null);
	let ownedUrl: string | null = null;
	let imgRetried = false;
	// Scrolled out of a virtual list mid-retry? Stop fetching for a tile nobody sees.
	let destroyed = false;

	const key = $derived(cacheKey(path, size, modTime));

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

	const MAX_PENDING_POLLS = 3;
	const MAX_THROTTLED_RETRIES = 5;

	// Wait as long as Retry-After asks (1-10s), plus jitter so a whole grid doesn't retry at once
	function throttleDelay(res: Response): number {
		const seconds = Number(res.headers.get("Retry-After"));
		const base = Number.isFinite(seconds) && seconds > 0 ? Math.min(seconds, 10) : 1;
		return base * 1000 + Math.random() * 500;
	}

	async function load() {
		loadState = "loading";
		let pending = 0;
		let throttled = 0;
		while (!destroyed) {
			try {
				const thumbUrl = `/api/thumbs${encodeFilePath(path)}?size=${size}${modTime ? `&v=${modTime}` : ""}`;
				const res = await fetch(thumbUrl, { credentials: "same-origin" });
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
				if (res.status === 202 && ++pending < MAX_PENDING_POLLS) {
					await new Promise((r) => setTimeout(r, 2000));
					continue;
				}
				// Rate limited is not broken: retry instead of caching a failure for the session
				if (res.status === 429 && ++throttled <= MAX_THROTTLED_RETRIES) {
					await new Promise((r) => setTimeout(r, throttleDelay(res)));
					continue;
				}
				fail();
				return;
			} catch {
				fail();
				return;
			}
		}
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
		// through to the children fallback (FileIcon), never the browser default.
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
		destroyed = true;
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
