<script lang="ts">
	import { onDestroy } from "svelte";
	import { getPreviewUrl } from "$lib/preview.js";
	import * as pdfjsLib from "pdfjs-dist";
	import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
	import ChevronLeftIcon from "@lucide/svelte/icons/chevron-left";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import MinusIcon from "@lucide/svelte/icons/minus";
	import PlusIcon from "@lucide/svelte/icons/plus";

	import DownloadIcon from "@lucide/svelte/icons/download";
	import XIcon from "@lucide/svelte/icons/x";
	import { Button } from "$lib/components/ui/button/index.js";
	import { viewport } from "$lib/stores/viewport.svelte.js";

	pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;

	let {
		path,
		url,
		name,
		ondownload,
		onclose,
	}: {
		path: string;
		url?: string;
		name?: string;
		ondownload?: () => void;
		onclose?: () => void;
	} = $props();

	let containerEl = $state<HTMLDivElement | null>(null);
	let pdfDoc = $state<pdfjsLib.PDFDocumentProxy | null>(null);
	let totalPages = $state(0);
	let currentPage = $state(1);
	let scale = $state(1);
	let fitToWidth = $state(false);
	let loading = $state(true);
	let error = $state("");

	let pageHeights = $state<number[]>([]);
	let baseHeights: number[] = [];
	let baseWidths: number[] = [];
	let firstPageBaseWidth = 0;
	let baseMaxWidth = 0;
	let pageProxies: pdfjsLib.PDFPageProxy[] = [];
	let renderedPages = new Set<number>();
	let renderingPages = new Set<number>();
	let renderTasks = new Map<number, pdfjsLib.RenderTask>();
	let renderGen = 0;
	let docGenCounter = 0;
	let docGen = $state(0);
	let canvasRefs = $state<(HTMLCanvasElement | null)[]>([]);
	let innerEl = $state<HTMLDivElement | null>(null);
	let pageRefs = $state<(HTMLDivElement | null)[]>([]);
	let observer: IntersectionObserver | null = null;
	let updateCurrentPageRaf: number | null = null;
	let renderScale = 1;
	let baseScale = 1;
	let layoutScale = $state(1);
	let pageRenderOrder: number[] = [];
	let showMobileToolbar = $state(true);
	let wasPinching = false;
	let wasPanning = false;
	const PAGE_BUFFER_SIZE = viewport.isMobile ? 5 : 10;
	const PAGE_GAP = 12;
	const DPR = Math.min(window.devicePixelRatio || 1, 2);

	let pageOffsets = $derived.by(() => {
		const offsets: number[] = [];
		let accum = 0;
		for (let i = 0; i < pageHeights.length; i++) {
			offsets.push(accum);
			accum += pageHeights[i] + PAGE_GAP;
		}
		return offsets;
	});
	let totalHeight = $derived(
		pageOffsets.length > 0
			? pageOffsets[pageOffsets.length - 1] + pageHeights[pageHeights.length - 1]
			: 0,
	);
	let contentWidth = $derived(baseMaxWidth > 0 ? Math.round(baseMaxWidth * layoutScale) : 0);

	function applyScale() {
		layoutScale = scale;
		pageHeights = baseHeights.map((h) => Math.round(h * scale));
	}

	function computeFitScale(): number {
		const w = containerEl?.clientWidth ?? 800;
		const hPad = viewport.isMobile ? 24 : 48;
		const fitW = (w - hPad) / firstPageBaseWidth;
		if (viewport.isMobile && baseHeights[0]) {
			const h = containerEl?.clientHeight ?? 600;
			const vPad = 120;
			const fitH = (h - vPad) / baseHeights[0];
			return Math.min(fitW, fitH);
		}
		return fitW;
	}

	async function renderPage(pageNum: number) {
		if (!pdfDoc || renderedPages.has(pageNum)) return;
		const page = pageProxies[pageNum - 1];
		const canvas = canvasRefs[pageNum - 1];
		if (!canvas || !page) return;

		renderTasks.get(pageNum)?.cancel();
		renderTasks.delete(pageNum);

		const gen = renderGen;
		renderingPages.add(pageNum);
		try {
			const pageViewport = page.getViewport({ scale: renderScale });
			const backingWidth = Math.round(pageViewport.width);
			const backingHeight = Math.round(pageViewport.height);

			const offscreen = document.createElement("canvas");
			offscreen.width = backingWidth;
			offscreen.height = backingHeight;

			const task = page.render({ canvas: offscreen, viewport: pageViewport });
			renderTasks.set(pageNum, task);
			await task.promise;

			if (gen !== renderGen) return;
			canvas.width = backingWidth;
			canvas.height = backingHeight;
			canvas.style.width = `${Math.round(baseWidths[pageNum - 1] * scale)}px`;
			canvas.style.height = `${Math.round(baseHeights[pageNum - 1] * scale)}px`;
			canvas.getContext("2d")!.drawImage(offscreen, 0, 0);
			offscreen.width = 1;
			offscreen.height = 1;
			renderedPages.add(pageNum);
			trackPageRender(pageNum);
		} catch (e) {
			if (e instanceof Error && e.name === "RenderingCancelledException") return;
			console.error(`Failed to render page ${pageNum}:`, e);
		} finally {
			renderingPages.delete(pageNum);
			renderTasks.delete(pageNum);
		}
	}

	function clearPage(pageNum: number) {
		renderTasks.get(pageNum)?.cancel();
		renderTasks.delete(pageNum);
		renderedPages.delete(pageNum);
		renderingPages.delete(pageNum);
		const canvas = canvasRefs[pageNum - 1];
		if (canvas) {
			canvas.width = 1;
			canvas.height = 1;
		}
		pageProxies[pageNum - 1]?.cleanup();
	}

	function trackPageRender(pageNum: number) {
		pageRenderOrder = pageRenderOrder.filter((p) => p !== pageNum);
		pageRenderOrder.push(pageNum);
		if (!containerEl) return;
		const cr = containerEl.getBoundingClientRect();
		while (pageRenderOrder.length > PAGE_BUFFER_SIZE) {
			const idx = pageRenderOrder.findIndex((p) => !isPageVisible(p, cr));
			if (idx === -1) break;
			const [page] = pageRenderOrder.splice(idx, 1);
			clearPage(page);
		}
	}

	function isPageVisible(pageNum: number, cr?: DOMRect): boolean {
		const ref = pageRefs[pageNum - 1];
		if (!ref || !containerEl) return false;
		const r = ref.getBoundingClientRect();
		const containerRect = cr ?? containerEl.getBoundingClientRect();
		return r.bottom > containerRect.top && r.top < containerRect.bottom;
	}

	function setupObserver() {
		if (observer) observer.disconnect();
		observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					const pageNum = Number((entry.target as HTMLElement).dataset.page);
					if (!pageNum) continue;
					if (entry.isIntersecting) {
						renderPage(pageNum);
					}
				}
				scheduleUpdateCurrentPage();
			},
			{ root: containerEl, rootMargin: "400px 0px", threshold: 0 },
		);
		for (const ref of pageRefs) {
			if (ref) observer.observe(ref);
		}
	}

	function scheduleUpdateCurrentPage() {
		if (updateCurrentPageRaf !== null) return;
		updateCurrentPageRaf = requestAnimationFrame(() => {
			updateCurrentPageRaf = null;
			updateCurrentPage();
		});
	}

	function updateCurrentPage() {
		if (!containerEl || pageOffsets.length === 0) return;
		const mid = containerEl.scrollTop + containerEl.clientHeight / 2;
		let lo = 0, hi = pageOffsets.length - 1;
		while (lo < hi) {
			const m = (lo + hi + 1) >>> 1;
			if (pageOffsets[m] <= mid) lo = m;
			else hi = m - 1;
		}
		currentPage = lo + 1;
	}

	function scrollToPage(pageNum: number) {
		renderPage(pageNum);
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

	const MAX_CANVAS_PIXELS = 8_000_000;

	function capRenderScale(target: number): number {
		if (!baseHeights[0]) return target;
		const pixels = (firstPageBaseWidth * target) * (baseHeights[0] * target);
		if (pixels <= MAX_CANVAS_PIXELS) return target;
		return Math.sqrt(MAX_CANVAS_PIXELS / (firstPageBaseWidth * baseHeights[0]));
	}

	function getTargetRenderScale(): number {
		if (viewport.isMobile) return capRenderScale(baseScale * 2);
		const percent = Math.round((scale / baseScale) * 100);
		const target = percent > 100 ? baseScale * 3 * DPR : baseScale * 2 * DPR;
		return capRenderScale(target);
	}

	function applyZoom() {
		for (let i = 0; i < canvasRefs.length; i++) {
			const canvas = canvasRefs[i];
			if (canvas && canvas.width > 1) {
				canvas.style.width = `${Math.round(baseWidths[i] * scale)}px`;
				canvas.style.height = `${Math.round(baseHeights[i] * scale)}px`;
			}
		}
		applyScale();

		const target = getTargetRenderScale();
		if (target !== renderScale) {
			renderScale = target;
			rerender();
		}
	}

	function zoom(direction: 1 | -1) {
		fitToWidth = false;
		const percent = Math.round((scale / baseScale) * 100);
		const step = 25;
		const next =
			direction > 0
				? Math.ceil((percent + 1) / step) * step
				: Math.floor((percent - 1) / step) * step;
		scale = Math.max(baseScale * 0.5, Math.min(baseScale * 5, (baseScale * next) / 100));
		applyZoom();
	}

	function displayPercent(): number {
		return Math.round((scale / baseScale) * 100);
	}

	function handleZoomInput(e: Event) {
		const input = e.target as HTMLInputElement;
		const val = parseInt(input.value, 10);
		if (isNaN(val)) {
			input.value = String(displayPercent());
			return;
		}
		const clamped = Math.max(50, Math.min(500, val));
		fitToWidth = false;
		scale = Math.max(baseScale * 0.5, Math.min(baseScale * 5, (baseScale * clamped) / 100));
		input.value = String(Math.round(displayPercent()));
		applyZoom();
	}

	function handleWheel(e: WheelEvent) {
		if (!e.ctrlKey) return;
		e.preventDefault();
		fitToWidth = false;

		const prevScale = scale;
		const factor = 1 - e.deltaY * 0.01;
		scale = Math.max(baseScale * 0.5, Math.min(baseScale * 5, scale * factor));
		const ratio = scale / prevScale;

		applyZoom();

		if (containerEl) {
			const rect = containerEl.getBoundingClientRect();
			const dx = e.clientX - rect.left;
			const dy = e.clientY - rect.top;
			containerEl.scrollLeft = (containerEl.scrollLeft + dx) * ratio - dx;
			containerEl.scrollTop = (containerEl.scrollTop + dy) * ratio - dy;
		}
	}

	function handleContainerClick() {
		if (!viewport.isMobile || wasPinching || wasPanning) return;
		showMobileToolbar = !showMobileToolbar;
	}

	function applyFitToWidth() {
		if (firstPageBaseWidth === 0) return;
		scale = computeFitScale();
		applyZoom();
	}

	function rerender() {
		renderGen++;
		for (const task of renderTasks.values()) task.cancel();
		renderTasks.clear();
		renderedPages.clear();
		renderingPages.clear();
		pageRenderOrder = [];
		applyScale();
		requestAnimationFrame(() => {
			if (observer) {
				observer.disconnect();
				setupObserver();
			}
		});
	}

	$effect(() => {
		let cancelled = false;
		loading = true;
		error = "";
		renderGen++;
		for (const task of renderTasks.values()) task.cancel();
		renderTasks.clear();
		renderedPages.clear();
		renderingPages.clear();
		canvasRefs = [];
		pageRefs = [];
		docGen = ++docGenCounter;
		baseHeights = [];
		baseWidths = [];
		firstPageBaseWidth = 0;
		baseMaxWidth = 0;
		pageProxies = [];

		const loadingTask = pdfjsLib.getDocument({
			url: url ?? getPreviewUrl(path),
			withCredentials: true,
		});

		loadingTask.promise
			.then(async (doc) => {
				if (cancelled) { doc.destroy(); return; }
				pdfDoc = doc;
				totalPages = doc.numPages;
				currentPage = 1;

				// Fetch all page proxies in parallel — pdf.js's worker handles
				// these concurrently. Cache base-scale dimensions so subsequent
				// zooms recompute heights via multiplication, never re-fetch.
				pageProxies = await Promise.all(
					Array.from({ length: doc.numPages }, (_, i) => doc.getPage(i + 1)),
				);
				if (cancelled) return;

				const viewports = pageProxies.map((p) => p.getViewport({ scale: 1 }));
				firstPageBaseWidth = viewports[0].width;
				baseWidths = viewports.map((v) => v.width);
				baseHeights = viewports.map((v) => v.height);
				baseMaxWidth = baseWidths.reduce((m, w) => Math.max(m, w), 0);

				baseScale = 1.25;
				scale = viewport.isMobile ? computeFitScale() : baseScale;
				renderScale = capRenderScale(baseScale * (viewport.isMobile ? 2 : 2 * DPR));
				applyScale();
				loading = false;

				requestAnimationFrame(() => {
					if (cancelled) return;
					const fitScale = computeFitScale();
					scale = viewport.isMobile ? fitScale : baseScale;
					renderScale = capRenderScale(baseScale * (viewport.isMobile ? 2 : 2 * DPR));
					applyScale();
					setupObserver();
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
			if (updateCurrentPageRaf !== null) {
				cancelAnimationFrame(updateCurrentPageRaf);
				updateCurrentPageRaf = null;
			}
			for (const task of renderTasks.values()) task.cancel();
			renderTasks.clear();
			if (pdfDoc) pdfDoc.destroy();
		};
	});

	let resizeTimer: ReturnType<typeof setTimeout> | null = null;
	function handleResize() {
		if (!fitToWidth || !pdfDoc) return;
		if (resizeTimer) clearTimeout(resizeTimer);
		resizeTimer = setTimeout(() => applyFitToWidth(), 150);
	}

	onDestroy(() => {
		if (resizeTimer) clearTimeout(resizeTimer);
	});

	// Set touch-action based on zoom state: pan-y when content fits horizontally
	// (native vertical scroll), none when zoomed in (manual free pan)
	$effect(() => {
		if (!containerEl || !viewport.isMobile) return;
		const el = containerEl;
		el.style.touchAction = contentWidth > el.clientWidth ? 'none' : 'pan-y';
		return () => { el.style.touchAction = ''; };
	});

	$effect(() => {
		if (!containerEl || !viewport.isMobile) return;
		const el = containerEl;
		let pinchStartDist = 0;
		let pinchStartScale = 1;
		let pinchMinScale = 0.5;
		let pinchMidX = 0;
		let pinchMidY = 0;
		let pinchOriginX = 0;
		let pinchOriginY = 0;
		let lastPinchMidX = 0;
		let lastPinchMidY = 0;
		let pinchPageIdx = 0;
		let pinchPageOffsetX = 0;
		let pinchPageOffsetY = 0;
		let panStartX = 0;
		let panStartY = 0;
		let panLastX = 0;
		let panLastY = 0;
		let panLastTime = 0;
		let vx = 0;
		let vy = 0;
		let momentumRaf = 0;
		let panning = false;

		function touchDist(a: Touch, b: Touch) {
			return Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
		}
		function stopMomentum() {
			if (momentumRaf) {
				cancelAnimationFrame(momentumRaf);
				momentumRaf = 0;
			}
		}
		function onTouchStart(e: TouchEvent) {
			stopMomentum();
			if (e.touches.length === 2) {
				wasPinching = true;
				panning = false;
				pinchStartDist = touchDist(e.touches[0], e.touches[1]);
				pinchStartScale = scale;
				pinchMinScale = computeFitScale();
				pinchMidX = (e.touches[0].clientX + e.touches[1].clientX) / 2;
				pinchMidY = (e.touches[0].clientY + e.touches[1].clientY) / 2;
				lastPinchMidX = pinchMidX;
				lastPinchMidY = pinchMidY;
				pinchPageIdx = currentPage - 1;
				const pageRef = pageRefs[pinchPageIdx];
				if (pageRef) {
					const pr = pageRef.getBoundingClientRect();
					pinchPageOffsetX = pinchMidX - pr.left;
					pinchPageOffsetY = pinchMidY - pr.top;
				}
				if (innerEl) {
					const rect = innerEl.getBoundingClientRect();
					pinchOriginX = pinchMidX - rect.left;
					pinchOriginY = pinchMidY - rect.top;
					innerEl.style.transformOrigin = `${pinchOriginX}px ${pinchOriginY}px`;
				}
			} else if (e.touches.length === 1) {
				panStartX = e.touches[0].clientX;
				panStartY = e.touches[0].clientY;
				panLastX = e.touches[0].clientX;
				panLastY = e.touches[0].clientY;
				panLastTime = performance.now();
				vx = 0;
				vy = 0;
				panning = false;
			}
		}
		function onTouchMove(e: TouchEvent) {
			if (e.touches.length === 2) {
				e.preventDefault();
				panning = false;
				const d = touchDist(e.touches[0], e.touches[1]);
				fitToWidth = false;
				scale = Math.max(pinchMinScale, Math.min(baseScale * 5, pinchStartScale * (d / pinchStartDist)));
				const mx = (e.touches[0].clientX + e.touches[1].clientX) / 2;
				const my = (e.touches[0].clientY + e.touches[1].clientY) / 2;
				lastPinchMidX = mx;
				lastPinchMidY = my;
				if (innerEl) {
					const tx = mx - pinchMidX;
					const ty = my - pinchMidY;
					innerEl.style.transform = `translate(${tx}px, ${ty}px) scale(${scale / pinchStartScale})`;
				}
				return;
			}
			if (e.touches.length !== 1) return;
			if (wasPinching) return;
			if (el.scrollWidth <= el.clientWidth + 1) return;

			const touch = e.touches[0];
			if (!panning) {
				const dist = Math.abs(touch.clientX - panStartX) + Math.abs(touch.clientY - panStartY);
				if (dist < 8) return;
				panning = true;
				wasPanning = true;
			}
			e.preventDefault();

			const dx = touch.clientX - panLastX;
			const dy = touch.clientY - panLastY;
			el.scrollLeft -= dx;
			el.scrollTop -= dy;

			const now = performance.now();
			const dt = now - panLastTime;
			if (dt > 0 && dt < 100) {
				vx = 0.4 * (dx / dt * 16) + 0.6 * vx;
				vy = 0.4 * (dy / dt * 16) + 0.6 * vy;
			}
			panLastX = touch.clientX;
			panLastY = touch.clientY;
			panLastTime = now;
		}
		function onTouchEnd(e: TouchEvent) {
			if (panning && e.touches.length === 0) {
				panning = false;
				if (performance.now() - panLastTime < 80) {
					const friction = 0.95;
					function step() {
						if (Math.abs(vx) < 0.5 && Math.abs(vy) < 0.5) return;
						el.scrollLeft -= vx;
						el.scrollTop -= vy;
						vx *= friction;
						vy *= friction;
						momentumRaf = requestAnimationFrame(step);
					}
					momentumRaf = requestAnimationFrame(step);
				}
				setTimeout(() => { wasPanning = false; }, 100);
			}
			if (wasPinching) {
				if (e.touches.length === 1) {
					panStartX = e.touches[0].clientX;
					panStartY = e.touches[0].clientY;
					panLastX = panStartX;
					panLastY = panStartY;
					panLastTime = performance.now();
					panning = false;
					vx = 0;
					vy = 0;
				}
				if (e.touches.length === 0) {
					const ratio = scale / pinchStartScale;
					const snapFitScale = computeFitScale();
					const shouldSnap = scale <= snapFitScale * 1.05;
					if (shouldSnap) {
						scale = snapFitScale;
						fitToWidth = true;
					}
					applyZoom();

					requestAnimationFrame(() => {
						if (!innerEl) return;
						innerEl.style.transform = '';
						innerEl.style.transformOrigin = '';
						const finalRatio = shouldSnap ? scale / pinchStartScale : ratio;
						const pageRef = pageRefs[pinchPageIdx];
						if (pageRef) {
							const pr = pageRef.getBoundingClientRect();
							const anchorX = pr.left + pinchPageOffsetX * finalRatio;
							const anchorY = pr.top + pinchPageOffsetY * finalRatio;
							el.scrollLeft += anchorX - lastPinchMidX;
							el.scrollTop += anchorY - lastPinchMidY;
						}
					});
				}
				setTimeout(() => { wasPinching = false; }, 100);
			}
		}
		el.addEventListener("touchstart", onTouchStart, { passive: true });
		el.addEventListener("touchmove", onTouchMove, { passive: false });
		el.addEventListener("touchend", onTouchEnd, { passive: true });
		return () => {
			stopMomentum();
			if (innerEl) {
				innerEl.style.transform = '';
				innerEl.style.transformOrigin = '';
			}
			el.removeEventListener("touchstart", onTouchStart);
			el.removeEventListener("touchmove", onTouchMove);
			el.removeEventListener("touchend", onTouchEnd);
		};
	});
</script>

<svelte:window onresize={handleResize} />

{#if loading}
	<div class="flex flex-1 items-center justify-center text-muted-foreground">
		<p class="text-[15px]">Loading PDF…</p>
	</div>
{:else if error}
	<div class="flex flex-1 items-center justify-center text-destructive">
		<p class="text-[15px]">{error}</p>
	</div>
{:else}
	<div class="relative flex flex-1 flex-col overflow-hidden bg-neutral-700" data-preview-content>
		{#if viewport.isMobile}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="absolute inset-x-0 top-0 z-10 flex items-center bg-zinc-800 px-2 py-2 transition-transform duration-200 ease-out"
				class:-translate-y-full={!showMobileToolbar}
				onclick={(e) => e.stopPropagation()}
			>
				<div class="flex-1"></div>
				<span class="text-xs tabular-nums text-zinc-400">{currentPage} / {totalPages}</span>
				<div class="flex flex-1 items-center justify-end gap-1">
					{#if ondownload}
						<Button variant="ghost" size="icon-touch" class="text-zinc-300"
							onclick={ondownload}
							title="Download"
							aria-label="Download"
						>
							<DownloadIcon class="size-5" />
						</Button>
					{/if}
					{#if onclose}
						<Button variant="ghost" size="icon-touch" class="text-zinc-200"
							onclick={onclose}
							title="Close"
							aria-label="Close preview"
						>
							<XIcon class="size-5" />
						</Button>
					{/if}
				</div>
			</div>
		{:else}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="flex items-center border-b border-border bg-zinc-900 px-3 py-1.5" onclick={(e) => e.stopPropagation()}>
				<div class="flex-1"></div>

				<div class="flex shrink-0 items-center gap-1.5">
					<Button variant="ghost" size="icon-touch" class="text-zinc-300"
						disabled={currentPage <= 1}
						onclick={() => { currentPage = Math.max(1, currentPage - 1); scrollToPage(currentPage); }}
						title="Previous page"
						aria-label="Previous page"
					>
						<ChevronLeftIcon class="size-5" />
					</Button>
					<div class="flex shrink-0 items-center gap-1 text-zinc-400">
						<input
							type="number"
							min="1"
							max={totalPages}
							value={currentPage}
							onchange={handlePageInput}
							class="w-12 rounded-md border border-transparent bg-zinc-800 px-1 py-0.5 text-center text-xs tabular-nums text-zinc-200 outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
						/>
						<span class="text-xs tabular-nums">/ {totalPages}</span>
					</div>
					<Button variant="ghost" size="icon-touch" class="text-zinc-300"
						disabled={currentPage >= totalPages}
						onclick={() => { currentPage = Math.min(totalPages, currentPage + 1); scrollToPage(currentPage); }}
						title="Next page"
						aria-label="Next page"
					>
						<ChevronRightIcon class="size-5" />
					</Button>

					<div class="mx-1 h-4 w-px shrink-0 bg-zinc-700"></div>

					<Button variant="ghost" size="icon-touch" class="text-zinc-300"
						onclick={() => zoom(-1)}
						title="Zoom out"
						aria-label="Zoom out"
					>
						<MinusIcon class="size-5" />
					</Button>
					<div class="flex shrink-0 items-center gap-0.5 text-zinc-400">
						<input
							type="number"
							min="50"
							max="500"
							value={displayPercent()}
							onchange={handleZoomInput}
							class="w-12 rounded-md border border-transparent bg-zinc-800 px-1 py-0.5 text-center text-xs tabular-nums text-zinc-200 outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
						/>
						<span class="text-xs tabular-nums">%</span>
					</div>
					<Button variant="ghost" size="icon-touch" class="text-zinc-300"
						onclick={() => zoom(1)}
						title="Zoom in"
						aria-label="Zoom in"
					>
						<PlusIcon class="size-5" />
					</Button>
				</div>

				<div class="flex flex-1 items-center justify-end gap-1">
					{#if ondownload}
						<Button variant="ghost" size="icon-touch" class="text-zinc-300"
							onclick={ondownload}
							title="Download"
							aria-label="Download"
						>
							<DownloadIcon class="size-5" />
						</Button>
					{/if}
					{#if onclose}
						<Button variant="ghost" size="icon-touch" class="text-zinc-200"
							onclick={onclose}
							title="Close"
							aria-label="Close preview"
						>
							<XIcon class="size-5" />
						</Button>
					{/if}
				</div>
			</div>
		{/if}

		<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
		<div
			bind:this={containerEl}
			data-pdf-scroll
			class="flex flex-1 flex-col overflow-auto overscroll-contain pt-16 pb-3 md:pt-4"
			onscroll={scheduleUpdateCurrentPage}
			onwheel={handleWheel}
			onclick={handleContainerClick}
		>
			<div bind:this={innerEl} class="relative my-auto min-w-full md:my-0" style="height: {totalHeight}px; width: {contentWidth}px">
				{#each Array(totalPages) as _, i (`${docGen}-${i}`)}
					<div
						bind:this={pageRefs[i]}
						data-page={i + 1}
						class="absolute overflow-hidden bg-white shadow-lg"
						style="contain: layout paint; top: {pageOffsets[i] ?? 0}px; height: {pageHeights[i] ?? 0}px; width: {Math.round((baseWidths[i] ?? 0) * layoutScale)}px; left: 50%; transform: translateX(-50%)"
					>
						<canvas bind:this={canvasRefs[i]} style="display: block;"></canvas>
					</div>
				{/each}
			</div>
		</div>
	</div>
{/if}

<style>
	:global([data-pdf-scroll]) {
		scrollbar-width: none;
	}
	:global([data-pdf-scroll])::-webkit-scrollbar {
		display: none;
	}
</style>
