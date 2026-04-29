<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import * as pdfjsLib from "pdfjs-dist";
	import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
	import ChevronLeftIcon from "@lucide/svelte/icons/chevron-left";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import ZoomInIcon from "@lucide/svelte/icons/zoom-in";
	import ZoomOutIcon from "@lucide/svelte/icons/zoom-out";
	import MaximizeIcon from "@lucide/svelte/icons/maximize";

	pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;

	let { path }: { path: string } = $props();

	let containerEl = $state<HTMLDivElement | null>(null);
	let pdfDoc = $state<pdfjsLib.PDFDocumentProxy | null>(null);
	let totalPages = $state(0);
	let currentPage = $state(1);
	let scale = $state(1);
	let fitToWidth = $state(true);
	let loading = $state(true);
	let error = $state("");

	let pageHeights = $state<number[]>([]);
	let renderedPages = new Set<number>();
	let renderingPages = new Set<number>();
	let canvasRefs: (HTMLCanvasElement | null)[] = [];
	let pageRefs: (HTMLDivElement | null)[] = [];
	let observer: IntersectionObserver | null = null;

	function calculateFitScale(viewport: { width: number }): number {
		const w = containerEl?.clientWidth ?? 800;
		return (w - 48) / viewport.width;
	}

	async function renderPage(pageNum: number) {
		if (!pdfDoc || renderedPages.has(pageNum) || renderingPages.has(pageNum)) return;
		const canvas = canvasRefs[pageNum - 1];
		if (!canvas) return;

		renderingPages.add(pageNum);
		try {
			const page = await pdfDoc.getPage(pageNum);
			const viewport = page.getViewport({ scale });
			canvas.width = viewport.width;
			canvas.height = viewport.height;
			canvas.style.width = `${viewport.width}px`;
			canvas.style.height = `${viewport.height}px`;

			await page.render({ canvas, viewport }).promise;
			renderedPages.add(pageNum);
		} finally {
			renderingPages.delete(pageNum);
		}
	}

	function setupObserver() {
		if (observer) observer.disconnect();
		observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						const pageNum = Number((entry.target as HTMLElement).dataset.page);
						if (pageNum) renderPage(pageNum);
					}
				}
				updateCurrentPage();
			},
			{ root: containerEl, rootMargin: "200px 0px", threshold: 0.1 },
		);
		for (const ref of pageRefs) {
			if (ref) observer.observe(ref);
		}
	}

	function updateCurrentPage() {
		if (!containerEl) return;
		const scrollTop = containerEl.scrollTop;
		const containerH = containerEl.clientHeight;
		const mid = scrollTop + containerH / 2;
		let accum = 0;
		for (let i = 0; i < pageHeights.length; i++) {
			accum += pageHeights[i] + 12; // 12px gap
			if (accum > mid) {
				currentPage = i + 1;
				return;
			}
		}
	}

	function scrollToPage(pageNum: number) {
		const ref = pageRefs[pageNum - 1];
		if (ref) ref.scrollIntoView({ behavior: "smooth", block: "start" });
	}

	function handlePageInput(e: Event) {
		const val = Number((e.target as HTMLInputElement).value);
		if (val >= 1 && val <= totalPages) {
			currentPage = val;
			scrollToPage(val);
		}
	}

	function zoom(delta: number) {
		fitToWidth = false;
		scale = Math.max(0.25, Math.min(5, scale + delta));
		rerender();
	}

	function toggleFitToWidth() {
		fitToWidth = !fitToWidth;
		if (fitToWidth) applyFitToWidth();
		else rerender();
	}

	async function applyFitToWidth() {
		if (!pdfDoc) return;
		const page = await pdfDoc.getPage(1);
		const viewport = page.getViewport({ scale: 1 });
		scale = calculateFitScale(viewport);
		rerender();
	}

	function rerender() {
		renderedPages.clear();
		renderingPages.clear();
		if (pdfDoc) {
			updatePageHeights();
		}
		// Re-render visible pages after heights update
		requestAnimationFrame(() => {
			if (observer) {
				observer.disconnect();
				setupObserver();
			}
		});
	}

	async function updatePageHeights() {
		if (!pdfDoc) return;
		const heights: number[] = [];
		for (let i = 1; i <= totalPages; i++) {
			const page = await pdfDoc.getPage(i);
			const viewport = page.getViewport({ scale });
			heights.push(viewport.height);
		}
		pageHeights = heights;
	}

	$effect(() => {
		let cancelled = false;
		loading = true;
		error = "";
		renderedPages.clear();
		renderingPages.clear();

		const loadingTask = pdfjsLib.getDocument({
			url: getPreviewUrl(path),
			withCredentials: true,
		});

		loadingTask.promise
			.then(async (doc) => {
				if (cancelled) { doc.destroy(); return; }
				pdfDoc = doc;
				totalPages = doc.numPages;
				currentPage = 1;

				const page = await doc.getPage(1);
				const viewport = page.getViewport({ scale: 1 });
				scale = calculateFitScale(viewport);

				await updatePageHeights();
				loading = false;

				// Wait for DOM to render page containers, then setup observer
				requestAnimationFrame(() => {
					if (!cancelled) setupObserver();
				});
			})
			.catch((e: unknown) => {
				if (cancelled) return;
				error = e instanceof Error ? e.message : "Failed to load PDF";
				loading = false;
			});

		return () => {
			cancelled = true;
			loadingTask.destroy();
			if (observer) observer.disconnect();
			if (pdfDoc) pdfDoc.destroy();
		};
	});

	let resizeTimer: ReturnType<typeof setTimeout> | null = null;
	function handleResize() {
		if (!fitToWidth || !pdfDoc) return;
		if (resizeTimer) clearTimeout(resizeTimer);
		resizeTimer = setTimeout(() => applyFitToWidth(), 150);
	}
</script>

<svelte:window onresize={handleResize} />

{#if loading}
	<div class="flex flex-1 items-center justify-center text-muted-foreground">
		<p class="text-sm">Loading PDF…</p>
	</div>
{:else if error}
	<div class="flex flex-1 items-center justify-center text-destructive">
		<p class="text-sm">{error}</p>
	</div>
{:else}
	<div class="flex flex-1 flex-col overflow-hidden">
		<div class="flex items-center gap-2 border-b border-border/50 bg-background/90 px-3 py-1.5 text-sm backdrop-blur-sm">
			<button
				class="rounded p-1 text-muted-foreground transition-colors hover:text-foreground disabled:opacity-30"
				disabled={currentPage <= 1}
				onclick={() => { currentPage = Math.max(1, currentPage - 1); scrollToPage(currentPage); }}
			>
				<ChevronLeftIcon class="size-4" />
			</button>
			<div class="flex items-center gap-1 text-muted-foreground">
				<input
					type="number"
					min="1"
					max={totalPages}
					value={currentPage}
					onchange={handlePageInput}
					class="w-10 rounded bg-accent px-1 py-0.5 text-center text-xs text-foreground"
				/>
				<span class="text-xs">/ {totalPages}</span>
			</div>
			<button
				class="rounded p-1 text-muted-foreground transition-colors hover:text-foreground disabled:opacity-30"
				disabled={currentPage >= totalPages}
				onclick={() => { currentPage = Math.min(totalPages, currentPage + 1); scrollToPage(currentPage); }}
			>
				<ChevronRightIcon class="size-4" />
			</button>

			<div class="mx-2 h-4 w-px bg-border/50"></div>

			<button
				class="rounded p-1 text-muted-foreground transition-colors hover:text-foreground"
				onclick={() => zoom(-0.25)}
			>
				<ZoomOutIcon class="size-4" />
			</button>
			<span class="min-w-[3rem] text-center text-xs text-muted-foreground">
				{Math.round(scale * 100)}%
			</span>
			<button
				class="rounded p-1 text-muted-foreground transition-colors hover:text-foreground"
				onclick={() => zoom(0.25)}
			>
				<ZoomInIcon class="size-4" />
			</button>

			<button
				class="rounded p-1 transition-colors"
				class:text-foreground={fitToWidth}
				class:text-muted-foreground={!fitToWidth}
				onclick={toggleFitToWidth}
				title="Fit to width"
			>
				<MaximizeIcon class="size-4" />
			</button>
		</div>

		<div
			bind:this={containerEl}
			class="flex flex-1 flex-col items-center gap-3 overflow-auto py-4"
			onscroll={updateCurrentPage}
		>
			{#each Array(totalPages) as _, i}
				<div
					bind:this={pageRefs[i]}
					data-page={i + 1}
					class="shrink-0 bg-white shadow-lg"
					style="height: {pageHeights[i] ?? 0}px"
				>
					<canvas bind:this={canvasRefs[i]}></canvas>
				</div>
			{/each}
		</div>
	</div>
{/if}
