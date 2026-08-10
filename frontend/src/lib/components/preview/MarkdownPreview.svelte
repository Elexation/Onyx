<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { marked } from "marked";
	import DOMPurify from "dompurify";

	let { path, url }: { path: string; url?: string } = $props();

	let srcdoc = $state("");
	let loading = $state(true);
	let error = $state("");

	const PROSE_STYLES = `
		html{color-scheme:dark}
		*,*::before,*::after{box-sizing:border-box}
		body{margin:0;padding:24px;background:#1e1e1e;color:#d4d4d4;font-family:ui-sans-serif,system-ui,-apple-system,sans-serif,"Apple Color Emoji","Segoe UI Emoji";font-size:14px;line-height:1.6;word-wrap:break-word;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale}
		h1,h2,h3,h4,h5,h6{color:#e5e5e5;margin:1.5em 0 0.5em;line-height:1.3;font-weight:600}
		h1{font-size:2em;border-bottom:1px solid #333;padding-bottom:0.3em}
		h2{font-size:1.5em;border-bottom:1px solid #333;padding-bottom:0.25em}
		h3{font-size:1.25em}h4{font-size:1em}
		p{margin:0.75em 0}
		a{color:#6ba3e8;text-decoration:underline}a:hover{color:#8bb9f0}
		code{font-family:ui-monospace,SFMono-Regular,"SF Mono",Menlo,Consolas,monospace;font-size:0.875em;background:#2d2d2d;padding:0.15em 0.35em;border-radius:3px}
		pre{background:#181818;padding:1em;border-radius:6px;overflow-x:auto;margin:1em 0}
		pre code{background:none;padding:0;font-size:0.85em;line-height:1.65}
		blockquote{margin:1em 0;padding:0.5em 1em;border-left:3px solid #444;color:#999}
		blockquote p{margin:0.25em 0}
		table{border-collapse:collapse;width:100%;margin:1em 0}
		th,td{border:1px solid #444;padding:0.5em 0.75em;text-align:left}
		th{background:#2a2a2a;font-weight:600}
		ul,ol{padding-left:1.5em;margin:0.75em 0}
		li{margin:0.25em 0}li>ul,li>ol{margin:0.25em 0}
		hr{border:none;border-top:1px solid #444;margin:1.5em 0}
		img{max-width:100%;height:auto;border-radius:4px}
	`;

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
			const sanitized = DOMPurify.sanitize(rendered, {
				FORBID_ATTR: ['onerror', 'onload', 'onmouseover', 'onfocus', 'onblur', 'onclick'],
				ALLOWED_URI_REGEXP: /^(?:https?|mailto|tel):/i,
			});
			srcdoc = `<!DOCTYPE html><html><head><meta charset="utf-8"><style>${PROSE_STYLES}</style></head><body><article>${sanitized}</article></body></html>`;
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
		<p class="text-[15px]">Loading…</p>
	</div>
{:else if error}
	<div class="flex flex-1 items-center justify-center text-destructive">
		<p class="text-[15px]">{error}</p>
	</div>
{:else}
	<div class="flex-1 overflow-hidden rounded-lg border border-border bg-[#1e1e1e]" data-preview-content>
		<iframe sandbox="" {srcdoc} class="h-full w-full border-0" title="Markdown preview"></iframe>
	</div>
{/if}
