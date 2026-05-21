<script lang="ts" module>
	type CacheState = "loaded" | "failed";
	type CacheEntry = { state: CacheState; url?: string };
	const cache = new Map<string, CacheEntry>();

	function cacheKey(path: string, size: string) {
		return size + "\0" + path;
	}
</script>

<script lang="ts">
	import { onDestroy } from "svelte";
	import type { Snippet } from "svelte";

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
	let state = $state<State>("idle");
	let url = $state<string | null>(null);
	let el: HTMLDivElement | null = $state(null);
	let ownedUrl: string | null = null;

	const key = $derived(cacheKey(path, size));

	$effect(() => {
		const cached = cache.get(key);
		if (cached) {
			state = cached.state;
			url = cached.url ?? null;
			return;
		}
		state = "idle";
		url = null;
	});

	$effect(() => {
		if (!el || state !== "idle") return;
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
		state = "loading";
		const maxAttempts = 3;
		for (let attempt = 1; attempt <= maxAttempts; attempt++) {
			try {
				const res = await fetch(
					`/api/thumbs${path}?size=${size}`,
					{ credentials: "same-origin" },
				);
				if (res.status === 200) {
					const blob = await res.blob();
					const objectUrl = URL.createObjectURL(blob);
					ownedUrl = objectUrl;
					url = objectUrl;
					state = "loaded";
					cache.set(key, { state: "loaded", url: objectUrl });
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
		state = "failed";
		url = null;
		cache.set(key, { state: "failed" });
	}

	onDestroy(() => {
		if (ownedUrl && cache.get(key)?.url !== ownedUrl) {
			URL.revokeObjectURL(ownedUrl);
		}
	});
</script>

<div bind:this={el} class={className}>
	{#if state === "loaded" && url}
		<img src={url} alt="" class="h-full w-full rounded object-cover" loading="lazy" />
	{:else}
		{@render children()}
	{/if}
</div>
