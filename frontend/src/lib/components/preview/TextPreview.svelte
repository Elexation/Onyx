<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { bundledLanguages, bundledThemes } from "shiki";
	import { createBundledHighlighter, createSingletonShorthands } from "shiki/core";
	import { createJavaScriptRegexEngine } from "shiki/engine/javascript";
	import DOMPurify from "dompurify";

	const { codeToHtml } = createSingletonShorthands(
		createBundledHighlighter({
			langs: bundledLanguages,
			themes: bundledThemes,
			engine: () => createJavaScriptRegexEngine({ forgiving: true }),
		}),
	);

	let { path, url }: { path: string; url?: string } = $props();

	let srcdoc = $state("");
	let loading = $state(true);
	let error = $state("");

	const HIGHLIGHT_SIZE_LIMIT = 256 * 1024;

	function escapeHtml(s: string): string {
		return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
	}

	const langMap: Record<string, string> = {
		".js": "javascript",
		".mjs": "javascript",
		".cjs": "javascript",
		".ts": "typescript",
		".tsx": "tsx",
		".jsx": "jsx",
		".json": "json",
		".py": "python",
		".go": "go",
		".rs": "rust",
		".rb": "ruby",
		".java": "java",
		".kt": "kotlin",
		".c": "c",
		".h": "c",
		".cpp": "cpp",
		".hpp": "cpp",
		".cs": "csharp",
		".css": "css",
		".scss": "scss",
		".html": "html",
		".xml": "xml",
		".svg": "xml",
		".yaml": "yaml",
		".yml": "yaml",
		".toml": "toml",
		".sh": "bash",
		".bash": "bash",
		".zsh": "bash",
		".fish": "fish",
		".ps1": "powershell",
		".sql": "sql",
		".php": "php",
		".lua": "lua",
		".r": "r",
		".swift": "swift",
		".dart": "dart",
		".zig": "zig",
		".vue": "vue",
		".svelte": "svelte",
		".dockerfile": "dockerfile",
		".makefile": "makefile",
		".mk": "makefile",
		".ini": "ini",
		".conf": "ini",
		".diff": "diff",
		".patch": "diff",
	};

	const CODE_STYLES = `
		html{color-scheme:dark}
		*,*::before,*::after{box-sizing:border-box}
		body{margin:0;padding:16px;background:#1e1e1e;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale}
		pre{margin:0;background:transparent!important}
		code{font-family:ui-monospace,SFMono-Regular,"SF Mono",Menlo,Consolas,"Liberation Mono",monospace;font-size:0.875rem;line-height:1.625}
		.line{display:inline-block;width:100%}
	`;

	function detectLang(filename: string): string {
		const dot = filename.lastIndexOf(".");
		if (dot === -1) return "text";
		const ext = filename.slice(dot).toLowerCase();
		return langMap[ext] ?? "text";
	}

	function wrapSrcdoc(body: string, extraCss = ""): string {
		return `<!DOCTYPE html><html><head><meta charset="utf-8"><style>${CODE_STYLES}${extraCss}</style></head><body>${body}</body></html>`;
	}

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
			const code = await res.text();
			if (signal.aborted) return;
			if (code.length > HIGHLIGHT_SIZE_LIMIT) {
				srcdoc = wrapSrcdoc(`<pre><code>${escapeHtml(code)}</code></pre>`, "code{color:#d4d4d4}");
			} else {
				const filename = p.split("/").pop() ?? p;
				const lang = detectLang(filename);
				const rendered = await codeToHtml(code, { lang, theme: "dark-plus" });
				if (signal.aborted) return;
				const sanitized = DOMPurify.sanitize(rendered, {
					FORBID_ATTR: ['onerror', 'onload', 'onmouseover', 'onfocus', 'onblur', 'onclick'],
					ALLOWED_URI_REGEXP: /^(?:https?|mailto|tel):/i,
				});
				srcdoc = wrapSrcdoc(sanitized);
			}
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
		<iframe sandbox="" {srcdoc} class="h-full w-full border-0" title="Code preview"></iframe>
	</div>
{/if}
