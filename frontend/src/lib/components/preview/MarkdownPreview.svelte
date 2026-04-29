<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { marked } from "marked";
	import DOMPurify from "dompurify";

	let { path }: { path: string } = $props();

	let html = $state("");
	let loading = $state(true);
	let error = $state("");

	$effect(() => {
		load(path);
	});

	async function load(p: string) {
		loading = true;
		error = "";
		try {
			const res = await fetch(getPreviewUrl(p), { credentials: "include" });
			if (!res.ok) throw new Error("Failed to load file");
			const raw = await res.text();
			const rendered = marked.parse(raw, { async: false }) as string;
			html = DOMPurify.sanitize(rendered);
		} catch (e) {
			error = e instanceof Error ? e.message : "Failed to load file";
		} finally {
			loading = false;
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
	<div class="flex-1 overflow-auto rounded-md bg-[#1e1e1e] p-6">
		<article class="prose prose-invert max-w-none">
			{@html html}
		</article>
	</div>
{/if}
