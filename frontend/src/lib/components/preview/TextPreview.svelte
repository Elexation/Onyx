<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { codeToHtml } from "shiki";

	let { path }: { path: string } = $props();

	let html = $state("");
	let loading = $state(true);
	let error = $state("");

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

	function detectLang(filename: string): string {
		const dot = filename.lastIndexOf(".");
		if (dot === -1) return "text";
		const ext = filename.slice(dot).toLowerCase();
		return langMap[ext] ?? "text";
	}

	$effect(() => {
		load(path);
	});

	async function load(p: string) {
		loading = true;
		error = "";
		try {
			const res = await fetch(getPreviewUrl(p), { credentials: "include" });
			if (!res.ok) throw new Error("Failed to load file");
			const code = await res.text();
			const filename = p.split("/").pop() ?? p;
			const lang = detectLang(filename);
			html = await codeToHtml(code, { lang, theme: "dark-plus" });
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
	<div class="preview-text flex-1 overflow-auto rounded-md bg-[#1e1e1e] p-4 text-sm">
		{@html html}
	</div>
{/if}

<style>
	.preview-text :global(pre) {
		margin: 0;
		background: transparent !important;
	}
	.preview-text :global(code) {
		font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
		font-size: 0.875rem;
		line-height: 1.625;
	}
	.preview-text :global(.line) {
		display: inline-block;
		width: 100%;
	}
</style>
