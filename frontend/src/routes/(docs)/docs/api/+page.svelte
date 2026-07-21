<script lang="ts">
	import { docsDrawer } from "$lib/stores/docsDrawer.svelte.js";
	import { scopeColors } from "$lib/docs/api-sections.js";
	import BookOpenIcon from "@lucide/svelte/icons/book-open";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import CheckIcon from "@lucide/svelte/icons/check";

	let copiedId = $state<string | null>(null);
	let copyTimer: ReturnType<typeof setTimeout> | null = null;

	async function copyToClipboard(text: string, id: string) {
		try {
			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(text);
			} else {
				const ta = document.createElement("textarea");
				ta.value = text;
				ta.style.position = "fixed";
				ta.style.opacity = "0";
				document.body.appendChild(ta);
				ta.focus();
				ta.select();
				const ok = document.execCommand("copy");
				document.body.removeChild(ta);
				if (!ok) throw new Error("copy failed");
			}
			copiedId = id;
			if (copyTimer) clearTimeout(copyTimer);
			copyTimer = setTimeout(() => (copiedId = null), 2000);
		} catch {
			// clipboard unavailable — button stays in default state
		}
	}

	const methodColors: Record<string, string> = {
		GET: "bg-accent-brand-dim text-accent-brand",
		HEAD: "bg-accent-brand-dim text-accent-brand",
		POST: "bg-muted text-foreground",
		PATCH: "bg-muted text-foreground",
		DELETE: "bg-destructive/15 text-destructive",
	};
</script>

{#snippet scopeBadge(scope: "read" | "upload" | "full")}
	<span
		class="shrink-0 rounded-[5px] px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] {scopeColors[scope]}"
	>
		{scope}
	</span>
{/snippet}

{#snippet methodBadge(method: string)}
	<span
		class="shrink-0 rounded-md px-2 py-0.5 font-mono text-[11px] font-semibold {methodColors[method] ?? 'bg-muted text-foreground'}"
	>
		{method}
	</span>
{/snippet}

{#snippet codeBlock(code: string, id: string)}
	<div class="group relative">
		<pre
			class="overflow-x-auto rounded-lg border border-border bg-muted/30 p-4 font-mono text-meta leading-relaxed text-foreground-dim"
		><code>{code}</code></pre>
		<button
			onclick={() => copyToClipboard(code, id)}
			class="absolute right-2 top-2 rounded-md bg-muted px-2 py-1 text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100 focus-visible:opacity-100"
			aria-label={copiedId === id ? "Copied" : "Copy to clipboard"}
		>
			<span class="relative block size-3.5 overflow-hidden">
				<CopyIcon
					class="absolute inset-0 size-3.5 transition-all duration-150 ease-out {copiedId === id ? '-translate-y-2 opacity-0' : 'translate-y-0 opacity-100'}"
				/>
				<CheckIcon
					class="absolute inset-0 size-3.5 text-accent-brand transition-all duration-150 ease-out {copiedId === id ? 'translate-y-0 opacity-100' : 'translate-y-2 opacity-0'}"
				/>
			</span>
		</button>
	</div>
{/snippet}

{#snippet paramRow(name: string, type: string, desc: string, required: boolean = false)}
	<tr class="border-b border-border last:border-b-0">
		<td class="px-3 py-2 font-mono">{name}{#if required}<span class="text-destructive">*</span>{/if}</td>
		<td class="px-3 py-2 text-muted-foreground">{type}</td>
		<td class="px-3 py-2 text-muted-foreground">{desc}</td>
	</tr>
{/snippet}

<div class="mx-auto max-w-[720px] p-6 md:px-8 md:py-7">
			<!-- Page header -->
			<header class="mb-6">
				<div class="flex items-center gap-2">
					<BookOpenIcon class="size-5 text-muted-foreground" strokeWidth={2} />
					<h1 class="text-[22px] font-bold tracking-[-0.01em]">API Reference</h1>
				</div>
				<p class="mt-1 text-meta text-muted-foreground">
					Use Personal Access Tokens to script against your Onyx instance. All endpoints below are
					accessible via bearer token authentication.
				</p>
			</header>

			<!-- ============================================ -->
			<!-- OVERVIEW -->
			<!-- ============================================ -->

			<!-- Authentication -->
			<section id="authentication" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("authentication")}>
				<h2 class="text-lg font-bold tracking-[-0.01em]">Authentication</h2>
				<p class="mt-2 text-sm leading-relaxed text-foreground-dim">
					All API requests require a Personal Access Token passed via the
					<code class="rounded bg-muted px-1.5 py-0.5 font-mono text-meta">Authorization</code>
					header using the Bearer scheme.
				</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_YOUR_TOKEN" \\\n  http://localhost:8080/api/files/', "auth-example")}
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Create and manage tokens in
					<a href="/settings" class="text-accent-brand hover:underline">Settings &rarr; Tokens</a>.
					Tokens are prefixed with
					<code class="rounded bg-muted px-1.5 py-0.5 font-mono text-meta">onyx_</code>
					and shown only once at creation.
				</p>
			</section>

			<!-- Scopes -->
			<section id="scopes" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("scopes")}>
				<h2 class="text-lg font-bold tracking-[-0.01em]">Scopes</h2>
				<p class="mt-2 mb-4 text-sm leading-relaxed text-foreground-dim">
					Each token has exactly one scope. Higher scopes include all lower permissions.
				</p>
				<div class="space-y-3">
					<div class="rounded-xl border border-border bg-card p-4">
						<div class="flex items-center gap-2">
							{@render scopeBadge("read")}
							<span class="text-sm font-medium">Read-only</span>
						</div>
						<p class="mt-1.5 text-meta text-muted-foreground">
							Browse files, download, search, view thumbnails, stream video, list shares, trash, and
							versions. GET and HEAD requests only.
						</p>
					</div>
					<div class="rounded-xl border border-border bg-card p-4">
						<div class="flex items-center gap-2">
							{@render scopeBadge("upload")}
							<span class="text-sm font-medium">Upload</span>
						</div>
						<p class="mt-1.5 text-meta text-muted-foreground">
							Everything in read, plus: upload files (tus protocol), create directories, and check
							upload conflicts.
						</p>
					</div>
					<div class="rounded-xl border border-border bg-card p-4">
						<div class="flex items-center gap-2">
							{@render scopeBadge("full")}
							<span class="text-sm font-medium">Full access</span>
						</div>
						<p class="mt-1.5 text-meta text-muted-foreground">
							Everything in upload, plus: rename, move, copy, delete files, manage shares, restore
							from trash, and manage versions.
						</p>
					</div>
				</div>
				<div class="mt-4 rounded-xl border border-border bg-card p-4">
					<p class="text-sm font-medium">Blocked endpoints</p>
					<p class="mt-1 text-meta text-muted-foreground">
						<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">/api/auth</code>,
						<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">/api/tokens</code>,
						<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">/api/settings</code>, and
						<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">/api/changes</code>
						require a browser session and are blocked for all token scopes, including full.
					</p>
				</div>
			</section>

			<!-- Errors & Conventions -->
			<section id="errors" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("errors")}>
				<h2 class="text-lg font-bold tracking-[-0.01em]">Errors & Conventions</h2>
				<p class="mt-2 text-sm leading-relaxed text-foreground-dim">
					All errors return a JSON object with a single
					<code class="rounded bg-muted px-1.5 py-0.5 font-mono text-meta">error</code> field:
				</p>
				{@render codeBlock('{ "error": "not found" }', "error-format")}
				<div class="mt-4 space-y-2 text-sm leading-relaxed text-foreground-dim">
					<p>
						<strong class="text-foreground">Timestamps</strong> are Unix seconds (not milliseconds).
					</p>
					<p>
						<strong class="text-foreground">Paths</strong> use forward slashes with a leading slash.
						The root directory is
						<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">/</code>.
					</p>
					<p>
						<strong class="text-foreground">Sizes</strong> are in bytes (int64 for files, uint64 for
						disk usage).
					</p>
				</div>
			</section>

			<!-- ============================================ -->
			<!-- READ SCOPE -->
			<!-- ============================================ -->

			<div class="mb-6 flex items-center gap-2 border-b border-border pb-3" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.filteredGroups.some((g) => g.scope === "read")}>
				<h2 class="text-lg font-bold tracking-[-0.01em]">Read Scope</h2>
				{@render scopeBadge("read")}
			</div>

			<!-- Files -->
			<section id="files" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("files")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/files/*</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					List a directory's contents or get metadata for a single file. The wildcard path maps to
					the file path on disk.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Query Parameters</h4>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Parameter</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Type</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							{@render paramRow("showHidden", "bool", "Include hidden files (dotfiles)")}
							{@render paramRow("dirsOnly", "bool", "Return only directories")}
						</tbody>
					</table>
				</div>

				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/files/Documents/"', "files-curl")}

				<h4 class="mt-4 text-sm font-semibold">Directory Response</h4>
				{@render codeBlock('{\n  "path": "/Documents",\n  "items": [\n    {\n      "name": "report.pdf",\n      "path": "/Documents/report.pdf",\n      "isDir": false,\n      "size": 1048576,\n      "modTime": 1714300000,\n      "mimeType": "application/pdf"\n    },\n    {\n      "name": "Photos",\n      "path": "/Documents/Photos",\n      "isDir": true,\n      "size": 0,\n      "modTime": 1714200000,\n      "itemCount": 42,\n      "hasSubDirs": true\n    }\n  ]\n}', "files-dir-resp")}

				<h4 class="mt-4 text-sm font-semibold">Single File Response</h4>
				{@render codeBlock('{\n  "name": "report.pdf",\n  "path": "/Documents/report.pdf",\n  "isDir": false,\n  "size": 1048576,\n  "modTime": 1714300000,\n  "mimeType": "application/pdf"\n}', "files-file-resp")}
			</section>

			<!-- Download -->
			<section id="download" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("download")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/download/*</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Download a single file. Returns the raw file bytes with
					<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">Content-Disposition: attachment</code>.
				</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" -OJ \\\n  "http://localhost:8080/api/download/Documents/report.pdf"', "download-curl")}

				<div class="mt-6 flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/download/zip</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Download multiple files as a ZIP archive. Pass paths as repeated query parameters. Maximum
					1,000 paths per request.
				</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" -o files.zip \\\n  "http://localhost:8080/api/download/zip?path=/docs/a.pdf&path=/docs/b.pdf"', "download-zip-curl")}
			</section>

			<!-- Preview -->
			<section id="preview" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("preview")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/preview/*</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Serve a file inline for browser preview. Returns
					<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">Content-Disposition: inline</code>.
					Potentially scriptable MIME types (HTML, SVG, XML) are sandboxed via CSP.
				</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/preview/Photos/sunset.jpg"', "preview-curl")}
			</section>

			<!-- Search -->
			<section id="search" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("search")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/search</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Full-text search across filenames. Returns a maximum of 20 results.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Query Parameters</h4>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Parameter</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Type</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							{@render paramRow("q", "string", "Search query", true)}
						</tbody>
					</table>
				</div>

				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/search?q=report"', "search-curl")}

				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "results": [\n    {\n      "name": "report.pdf",\n      "path": "/Documents/report.pdf",\n      "isDir": false\n    }\n  ],\n  "total": 1\n}', "search-resp")}
			</section>

			<!-- Thumbnails -->
			<section id="thumbnails" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("thumbnails")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/thumbs/*</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Get a JPEG thumbnail for an image or video file.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Query Parameters</h4>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Parameter</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Type</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							{@render paramRow("size", "string", '"small", "medium" (default), or "large"')}
						</tbody>
					</table>
				</div>

				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" -o thumb.jpg \\\n  "http://localhost:8080/api/thumbs/Photos/sunset.jpg?size=large"', "thumbs-curl")}

				<div class="mt-3 space-y-1 text-sm text-foreground-dim">
					<p><strong class="text-foreground">200</strong> &mdash; JPEG thumbnail bytes</p>
					<p>
						<strong class="text-foreground">202</strong> &mdash; Thumbnail queued for generation.
						Retry after 2 seconds (<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">Retry-After: 2</code>
						header).
					</p>
					<p>
						<strong class="text-foreground">415</strong> &mdash; Unsupported file type or generation
						failed
					</p>
				</div>
			</section>

			<!-- Streaming -->
			<section id="streaming" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("streaming")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/stream/...</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					HLS video streaming endpoints. Requires ffmpeg/ffprobe on the server. Returns
					<strong class="text-foreground">501</strong> if unavailable.
				</p>

				<h4 class="mt-5 text-sm font-semibold">Stream Info</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">GET /api/stream/info/*</code> &mdash; Returns video metadata.
				</p>
				{@render codeBlock('{\n  "codec": "h264",\n  "width": 1920,\n  "height": 1080,\n  "duration": 120.5,\n  "bitrate": 5000000,\n  "framerate": 29.97,\n  "needsTranscode": false\n}', "stream-info-resp")}

				<h4 class="mt-5 text-sm font-semibold">Playlists & Segments</h4>
				<div class="mt-2 space-y-1 text-meta text-foreground-dim">
					<p>
						<code class="font-mono">GET /api/stream/master/*</code> &mdash; HLS master playlist (m3u8)
					</p>
					<p>
						<code class="font-mono">GET /api/stream/playlist/&#123;v&#125;/*</code> &mdash; Variant
						playlist. <code class="font-mono">v</code> = variant index (0, 1, 2...).
					</p>
					<p>
						<code class="font-mono">GET /api/stream/init/&#123;v&#125;/*</code> &mdash; fMP4 init
						segment. Blocks up to 30s while ffmpeg produces it.
					</p>
					<p>
						<code class="font-mono">GET /api/stream/segment/&#123;v&#125;/&#123;n&#125;/*</code>
						&mdash; Media segment.
						<code class="font-mono">n</code> = segment index (0-based). Blocks up to 30s.
					</p>
				</div>
			</section>

			<!-- Shares (read) -->
			<section id="shares-read" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("shares-read")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/shares/</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">List all share links.</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/shares/"', "shares-list-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "shares": [\n    {\n      "id": 1,\n      "tokenLast8": "abc12345",\n      "filePath": "/Documents/report.pdf",\n      "isDir": false,\n      "createdAt": 1714300000,\n      "expiresAt": 1714900000,\n      "hasPassword": true,\n      "downloadCount": 5\n    }\n  ]\n}', "shares-list-resp")}

				<div class="mt-6 flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/shares/by-path</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Look up a share by file path. Returns
					<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">null</code> if no share exists.
				</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/shares/by-path?path=/Documents/report.pdf"', "shares-bypath-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "share": {\n    "id": 1,\n    "tokenLast8": "abc12345",\n    "filePath": "/Documents/report.pdf",\n    "isDir": false,\n    "createdAt": 1714300000,\n    "hasPassword": false,\n    "downloadCount": 12\n  }\n}', "shares-bypath-resp")}

				<div class="mt-6 flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/shares/count</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">Get total number of active shares.</p>
				{@render codeBlock('{ "count": 5 }', "shares-count-resp")}
			</section>

			<!-- Trash (read) -->
			<section id="trash-read" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("trash-read")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/trash/</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">List all items in the trash.</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/trash/"', "trash-list-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "items": [\n    {\n      "id": "a1b2c3d4e5f6",\n      "originalPath": "/Documents/old-report.pdf",\n      "deletedAt": 1714200000,\n      "size": 524288,\n      "isDir": false\n    }\n  ],\n  "count": 1\n}', "trash-list-resp")}

				<div class="mt-6 flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/trash/count</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">Get total number of trashed items.</p>
				{@render codeBlock('{ "count": 3 }', "trash-count-resp")}
			</section>

			<!-- Versions (read) -->
			<section id="versions-read" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("versions-read")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/versions/</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					List version history for a specific file.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Query Parameters</h4>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Parameter</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Type</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							{@render paramRow("path", "string", "File path to list versions for", true)}
						</tbody>
					</table>
				</div>

				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/versions/?path=/Documents/report.pdf"', "versions-list-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "items": [\n    {\n      "id": 1,\n      "filePath": "/Documents/report.pdf",\n      "createdAt": 1714200000,\n      "size": 524288\n    }\n  ]\n}', "versions-list-resp")}

				<div class="mt-6 flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/versions/count</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Get total number of stored versions across all files.
				</p>
				{@render codeBlock('{ "count": 10 }', "versions-count-resp")}
			</section>

			<!-- Storage -->
			<section id="storage" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("storage")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("GET")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/storage</code>
					{@render scopeBadge("read")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Get disk usage for the data directory's host filesystem.
				</p>
				{@render codeBlock('curl -H "Authorization: Bearer onyx_TOKEN" \\\n  "http://localhost:8080/api/storage"', "storage-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "used": 10737418240,\n  "total": 107374182400\n}', "storage-resp")}
				<p class="mt-2 text-meta text-muted-foreground">Values are in bytes (uint64).</p>
			</section>

			<!-- ============================================ -->
			<!-- UPLOAD SCOPE -->
			<!-- ============================================ -->

			<div class="mb-6 flex items-center gap-2 border-b border-border pb-3" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.filteredGroups.some((g) => g.scope === "upload")}>
				<h2 class="text-lg font-bold tracking-[-0.01em]">Upload Scope</h2>
				{@render scopeBadge("upload")}
			</div>

			<!-- Check Conflicts -->
			<section id="check-conflicts" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("check-conflicts")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					<code class="font-mono text-[15px] font-medium text-foreground"
						>/api/files/check-conflicts</code
					>
					{@render scopeBadge("upload")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Check if files already exist at the target location before uploading. Maximum 500 paths per
					request.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				{@render codeBlock('{\n  "targetDir": "/Documents",\n  "paths": ["report.pdf", "notes.txt"]\n}', "conflicts-req")}

				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "conflicts": [\n    {\n      "path": "report.pdf",\n      "size": 1048576,\n      "modTime": 1714300000\n    }\n  ]\n}', "conflicts-resp")}
				<p class="mt-2 text-meta text-muted-foreground">
					Only existing files appear in the conflicts array. If no conflicts, the array is empty.
				</p>
			</section>

			<!-- Make Directory -->
			<section id="mkdir" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("mkdir")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/files/mkdir</code>
					{@render scopeBadge("upload")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">Create a new directory.</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				{@render codeBlock('{ "path": "/Documents/New Folder" }', "mkdir-req")}

				<h4 class="mt-4 text-sm font-semibold">Response <span class="font-normal text-muted-foreground">(201)</span></h4>
				{@render codeBlock('{ "path": "/Documents/New Folder" }', "mkdir-resp")}
				<p class="mt-2 text-meta text-muted-foreground">
					Returns <strong>409</strong> if the directory already exists.
				</p>
			</section>

			<!-- Upload (tus) -->
			<section id="upload" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("upload")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					{@render methodBadge("PATCH")}
					{@render methodBadge("HEAD")}
					{@render methodBadge("DELETE")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/upload/*</code>
					{@render scopeBadge("upload")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Resumable file uploads via the
					<a
						href="https://tus.io/protocols/resumable-upload"
						target="_blank"
						rel="noopener noreferrer"
						class="text-accent-brand hover:underline">tus 1.0.0 protocol</a
					>. Supports creation, creation-with-upload, and termination extensions.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Required Metadata</h4>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Key</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Required</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							<tr class="border-b border-border">
								<td class="px-3 py-2 font-mono">name</td>
								<td class="px-3 py-2 text-muted-foreground">Yes</td>
								<td class="px-3 py-2 text-muted-foreground">Filename</td>
							</tr>
							<tr class="border-b border-border">
								<td class="px-3 py-2 font-mono">targetDir</td>
								<td class="px-3 py-2 text-muted-foreground">Yes</td>
								<td class="px-3 py-2 text-muted-foreground">Destination directory path</td>
							</tr>
							<tr class="border-b border-border">
								<td class="px-3 py-2 font-mono">relativePath</td>
								<td class="px-3 py-2 text-muted-foreground">No</td>
								<td class="px-3 py-2 text-muted-foreground"
									>Path relative to targetDir (for folder uploads with subdirectories). Defaults to
									filename.</td
								>
							</tr>
							<tr class="last:border-b-0">
								<td class="px-3 py-2 font-mono">conflictStrategy</td>
								<td class="px-3 py-2 text-muted-foreground">No</td>
								<td class="px-3 py-2 text-muted-foreground"
									><code class="rounded bg-muted px-1 py-0.5 font-mono">"replace"</code> (overwrite,
									versions old file) or
									<code class="rounded bg-muted px-1 py-0.5 font-mono">"keepBoth"</code>
									(auto-rename). Omit to fail on conflict (409).</td
								>
							</tr>
						</tbody>
					</table>
				</div>

				<h4 class="mt-4 text-sm font-semibold">Example: Create Upload</h4>
				{@render codeBlock('curl -X POST "http://localhost:8080/api/upload/" \\\n  -H "Authorization: Bearer onyx_TOKEN" \\\n  -H "Tus-Resumable: 1.0.0" \\\n  -H "Upload-Length: 1048576" \\\n  -H "Upload-Metadata: name dGVzdC5wZGY=,targetDir Lw==" \\\n  -H "Content-Length: 0"', "upload-create-curl")}
				<p class="mt-2 text-meta text-muted-foreground">
					Metadata values are base64-encoded. The example above uploads
					<code class="rounded bg-muted px-1 py-0.5 font-mono">test.pdf</code> to
					<code class="rounded bg-muted px-1 py-0.5 font-mono">/</code>.
					See the <a
						href="https://tus.io/protocols/resumable-upload"
						target="_blank"
						rel="noopener noreferrer"
						class="text-accent-brand hover:underline">tus protocol spec</a
					> for the full PATCH/HEAD/DELETE flow.
				</p>

				<div class="mt-4 space-y-1 text-sm text-foreground-dim">
					<p>
						<strong class="text-foreground">Upload-Defer-Length</strong> is rejected &mdash; the
						server requires an upfront size for enforcement.
					</p>
					<p>
						<strong class="text-foreground">Size limit</strong> is configurable via the
						<code class="rounded bg-muted px-1 py-0.5 font-mono text-meta">upload.max_size</code>
						setting (0 = unlimited). Exceeding it returns <strong>413</strong>.
					</p>
					<p>
						<strong class="text-foreground">Concurrency</strong> is limited to 8 concurrent uploads
						per IP.
					</p>
				</div>
			</section>

			<!-- ============================================ -->
			<!-- FULL SCOPE -->
			<!-- ============================================ -->

			<div class="mb-6 flex items-center gap-2 border-b border-border pb-3" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.filteredGroups.some((g) => g.scope === "full")}>
				<h2 class="text-lg font-bold tracking-[-0.01em]">Full Scope</h2>
				{@render scopeBadge("full")}
			</div>

			<!-- Rename -->
			<section id="rename" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("rename")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/files/rename</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Rename a file or directory in place.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				{@render codeBlock('{\n  "path": "/Documents/old-name.pdf",\n  "newName": "new-name.pdf"\n}', "rename-req")}

				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{ "status": "ok" }', "rename-resp")}
				<p class="mt-2 text-meta text-muted-foreground">
					Returns <strong>409</strong> if a file with the new name already exists.
				</p>
			</section>

			<!-- Move -->
			<section id="move" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("move")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/files/move</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Move files or directories to a new location. Maximum 500 paths per request. Each item
					reports success independently.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				{@render codeBlock('{\n  "paths": ["/Documents/a.pdf", "/Documents/b.pdf"],\n  "destination": "/Archive"\n}', "move-req")}

				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "results": [\n    { "path": "/Documents/a.pdf", "success": true },\n    { "path": "/Documents/b.pdf", "success": true }\n  ]\n}', "move-resp")}
			</section>

			<!-- Copy -->
			<section id="copy" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("copy")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/files/copy</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Copy files or directories. Maximum 500 paths per request.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				{@render codeBlock('{\n  "paths": ["/Documents/report.pdf"],\n  "destination": "/Backup"\n}', "copy-req")}

				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "results": [\n    { "path": "/Documents/report.pdf", "success": true }\n  ]\n}', "copy-resp")}
			</section>

			<!-- Delete -->
			<section id="delete" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("delete")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("DELETE")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/files/</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Delete files or directories. Maximum 500 paths per request.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				{@render codeBlock('{\n  "paths": ["/Documents/old-report.pdf"],\n  "permanent": false\n}', "delete-req")}
				<p class="mt-2 text-meta text-muted-foreground">
					<code class="rounded bg-muted px-1 py-0.5 font-mono">permanent: false</code> moves to
					trash (if enabled).
					<code class="rounded bg-muted px-1 py-0.5 font-mono">permanent: true</code> deletes
					immediately and cannot be undone.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{\n  "results": [\n    { "path": "/Documents/old-report.pdf", "success": true }\n  ]\n}', "delete-resp")}
			</section>

			<!-- Create Share -->
			<section id="shares-write" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("shares-write")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/shares/</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Create a public share link for a file or directory. Only one share per path is allowed.
				</p>

				<h4 class="mt-4 text-sm font-semibold">Request Body</h4>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Field</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Type</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							{@render paramRow("path", "string", "File or directory path", true)}
							{@render paramRow("isDir", "bool", "Whether the path is a directory", true)}
							{@render paramRow("expiresIn", "string", 'Go duration string, e.g. "24h", "168h". Omit for no expiry.')}
							{@render paramRow("password", "string", "Optional password protection")}
						</tbody>
					</table>
				</div>

				{@render codeBlock('curl -X POST "http://localhost:8080/api/shares/" \\\n  -H "Authorization: Bearer onyx_TOKEN" \\\n  -H "Content-Type: application/json" \\\n  -d \'{"path": "/Documents/report.pdf", "isDir": false, "expiresIn": "168h"}\'', "shares-create-curl")}

				<h4 class="mt-4 text-sm font-semibold">Response <span class="font-normal text-muted-foreground">(201)</span></h4>
				{@render codeBlock('{\n  "id": 1,\n  "token": "full-64-char-share-token",\n  "tokenLast8": "abc12345",\n  "filePath": "/Documents/report.pdf",\n  "isDir": false,\n  "createdAt": 1714300000,\n  "expiresAt": 1714904000,\n  "hasPassword": false,\n  "downloadCount": 0\n}', "shares-create-resp")}
				<p class="mt-2 text-meta text-muted-foreground">
					The full <code class="rounded bg-muted px-1 py-0.5 font-mono">token</code> is only returned
					at creation and cannot be retrieved later. Returns <strong>409</strong> if a share already
					exists for this path.
				</p>
			</section>

			<!-- Delete Share -->
			<section id="shares-delete" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("shares-delete")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("DELETE")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/shares/&#123;id&#125;</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">
					Revoke a share link by its numeric ID.
				</p>
				{@render codeBlock('curl -X DELETE "http://localhost:8080/api/shares/1" \\\n  -H "Authorization: Bearer onyx_TOKEN"', "shares-delete-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{ "status": "deleted" }', "shares-delete-resp")}
			</section>

			<!-- Trash Operations -->
			<section id="trash-write" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("trash-write")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					{@render methodBadge("DELETE")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/trash/...</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">Manage trashed items.</p>

				<h4 class="mt-5 text-sm font-semibold">Restore Item</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">POST /api/trash/&#123;id&#125;/restore</code>
				</p>
				<div class="mt-2 overflow-hidden rounded-lg border border-border">
					<table class="w-full text-meta">
						<thead>
							<tr class="border-b border-border bg-muted/30">
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Query Param</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Values</th>
								<th class="px-3 py-2 text-left font-medium text-muted-foreground">Description</th>
							</tr>
						</thead>
						<tbody>
							{@render paramRow("strategy", "string", '"replace", "keepBoth", "skip", or omit')}
						</tbody>
					</table>
				</div>
				{@render codeBlock('curl -X POST "http://localhost:8080/api/trash/a1b2c3d4/restore?strategy=replace" \\\n  -H "Authorization: Bearer onyx_TOKEN"', "trash-restore-curl")}
				<h4 class="mt-4 text-sm font-semibold">Response</h4>
				{@render codeBlock('{ "status": "restored", "path": "/Documents/report.pdf" }', "trash-restore-resp")}
				<p class="mt-2 text-meta text-muted-foreground">
					With <code class="rounded bg-muted px-1 py-0.5 font-mono">strategy=skip</code>, returns
					<code class="rounded bg-muted px-1 py-0.5 font-mono">&#123; "status": "skipped" &#125;</code>.
					Returns <strong>409</strong> if no strategy is specified and a conflict exists.
				</p>

				<h4 class="mt-5 text-sm font-semibold">Check Restore Conflicts</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">POST /api/trash/check-restore-conflicts</code>
				</p>
				{@render codeBlock('// Request\n{ "ids": ["a1b2c3d4", "e5f6g7h8"] }\n\n// Response\n{\n  "conflicts": [\n    {\n      "id": "a1b2c3d4",\n      "path": "/Documents/report.pdf",\n      "isDir": false,\n      "existing": { "size": 2048, "modTime": 1714300000 },\n      "restoring": { "size": 1024, "modTime": 1714200000 }\n    }\n  ]\n}', "trash-conflicts-resp")}
				<p class="mt-2 text-meta text-muted-foreground">Maximum 500 IDs per request.</p>

				<h4 class="mt-5 text-sm font-semibold">Permanently Delete Item</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">DELETE /api/trash/&#123;id&#125;</code> &mdash; Permanently removes a
					single trashed item.
				</p>
				{@render codeBlock('{ "status": "deleted" }', "trash-purge-resp")}

				<h4 class="mt-5 text-sm font-semibold">Empty Trash</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">DELETE /api/trash/</code> &mdash; Permanently removes all trashed
					items.
				</p>
				{@render codeBlock('{ "status": "emptied" }', "trash-empty-resp")}
			</section>

			<!-- Version Operations -->
			<section id="versions-write" class="scroll-mt-6 mb-10" class:hidden={docsDrawer.filterQuery.trim() && !docsDrawer.matchedIds.has("versions-write")}>
				<div class="flex flex-wrap items-center gap-2">
					{@render methodBadge("POST")}
					{@render methodBadge("DELETE")}
					<code class="font-mono text-[15px] font-medium text-foreground">/api/versions/...</code>
					{@render scopeBadge("full")}
				</div>
				<p class="mt-3 text-sm leading-relaxed text-foreground-dim">Manage file versions.</p>

				<h4 class="mt-5 text-sm font-semibold">Restore Version</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">POST /api/versions/&#123;id&#125;/restore</code> &mdash; Replaces the
					current file with this version. The current file becomes a new version entry.
				</p>
				{@render codeBlock('curl -X POST "http://localhost:8080/api/versions/1/restore" \\\n  -H "Authorization: Bearer onyx_TOKEN"', "version-restore-curl")}
				{@render codeBlock('{ "status": "restored" }', "version-restore-resp")}

				<h4 class="mt-5 text-sm font-semibold">Delete Version</h4>
				<p class="mt-1 text-meta text-muted-foreground">
					<code class="font-mono">DELETE /api/versions/&#123;id&#125;</code> &mdash; Permanently deletes
					a specific version.
				</p>
				{@render codeBlock('{ "status": "deleted" }', "version-delete-resp")}
			</section>

			{#if docsDrawer.filterQuery.trim() && docsDrawer.matchedIds.size === 0}
				<div class="flex flex-col items-center justify-center py-20 text-center">
					<p class="text-sm text-muted-foreground">No endpoints match your filter.</p>
				</div>
			{/if}
		</div>
