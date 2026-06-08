<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { marked } from "marked";
	import DOMPurify from "dompurify";

	let { path, url }: { path: string; url?: string } = $props();

	let html = $state("");
	let loading = $state(true);
	let error = $state("");

	$effect(() => {
		const controller = new AbortController();
		load(path, controller.signal);
		return () => controller.abort();
	});

	async function load(p: string, signal: AbortSignal) {
		loading = true;
		error = "";
		try {
			const res = await fetch(url ?? getPreviewUrl(p), { credentials: "include", signal });
			if (!res.ok) throw new Error("Failed to load file");
			const raw = await res.text();
			if (signal.aborted) return;
			const rendered = marked.parse(raw, { async: false }) as string;
			if (signal.aborted) return;
			html = DOMPurify.sanitize(rendered);
		} catch (e) {
			if (signal.aborted || (e instanceof DOMException && e.name === "AbortError")) return;
			error = e instanceof Error ? e.message : "Failed to load file";
		} finally {
			if (!signal.aborted) loading = false;
		}
	}
</script>

{#if loading}
	<div class="flex flex-1 items-center justify-center text-muted-foreground">
		<p class="text-sm">Loading…</p>
	</div>
{:else if error}
	<div class="flex flex-1 items-center justify-center text-destructive">
		<p class="text-sm">{error}</p>
	</div>
{:else}
	<div class="flex-1 overflow-auto rounded-md bg-[#1e1e1e] p-6" data-preview-content>
		<article class="prose prose-invert max-w-none">
			{@html html}
		</article>
	</div>
{/if}
