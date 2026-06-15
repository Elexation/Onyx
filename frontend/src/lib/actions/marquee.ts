import { selection } from "$lib/stores/selection.svelte.js";

interface GridLayout {
	mode: "grid";
	itemWidth: number;
	itemHeight: number;
	gap: number;
	paddingX: number;
	columns: number;
}

interface ListLayout {
	mode: "list";
	rowHeight: number;
	/** Pixels of non-row content (e.g. header) at the top of the scroll container before row 0. */
	headerOffset?: number;
}

interface MarqueeParams {
	getLayout: () => GridLayout | ListLayout;
	getItems: () => Array<{ path: string; name: string }>;
}

interface Rect {
	x: number;
	y: number;
	w: number;
	h: number;
}

const DEAD_ZONE = 5;
const EDGE_ZONE = 40;
const MAX_SCROLL_SPEED = 15;

function rectsOverlap(a: Rect, b: Rect): boolean {
	return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
}

function isOnInteractive(target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement)) return false;
	return target.closest(
		'[role="gridcell"], [role="row"], button, a, input, label, [data-bits-menu-content]',
	) !== null;
}

function toContentCoords(e: PointerEvent, scrollEl: HTMLElement): { x: number; y: number } {
	const rect = scrollEl.getBoundingClientRect();
	return {
		x: e.clientX - rect.left + scrollEl.scrollLeft,
		y: e.clientY - rect.top + scrollEl.scrollTop,
	};
}

function hitTestGrid(marquee: Rect, layout: GridLayout, itemCount: number): Set<number> {
	const { itemWidth, itemHeight, gap, paddingX, columns } = layout;
	const colStride = itemWidth + gap;
	const rowStride = itemHeight + gap;
	const indices = new Set<number>();

	const colStart = Math.max(0, Math.floor((marquee.x - paddingX) / colStride));
	const colEnd = Math.min(columns - 1, Math.floor((marquee.x + marquee.w - paddingX) / colStride));
	const rowStart = Math.max(0, Math.floor(marquee.y / rowStride));
	const rowEnd = Math.floor((marquee.y + marquee.h) / rowStride);

	for (let row = rowStart; row <= rowEnd; row++) {
		for (let col = colStart; col <= colEnd; col++) {
			const idx = row * columns + col;
			if (idx >= itemCount) continue;
			const itemRect: Rect = {
				x: paddingX + col * colStride,
				y: row * rowStride,
				w: itemWidth,
				h: itemHeight,
			};
			if (rectsOverlap(marquee, itemRect)) {
				indices.add(idx);
			}
		}
	}
	return indices;
}

function hitTestList(marquee: Rect, layout: ListLayout, itemCount: number): Set<number> {
	const { rowHeight, headerOffset = 0 } = layout;
	const indices = new Set<number>();

	const top = marquee.y - headerOffset;
	const bottom = marquee.y + marquee.h - headerOffset;
	const rowStart = Math.max(0, Math.floor(top / rowHeight));
	const rowEnd = Math.min(itemCount - 1, Math.floor(bottom / rowHeight));

	for (let i = rowStart; i <= rowEnd; i++) {
		const itemTop = i * rowHeight;
		if (itemTop + rowHeight > top && itemTop < bottom) {
			indices.add(i);
		}
	}
	return indices;
}

export function setupMarquee(scrollEl: HTMLDivElement, params: MarqueeParams): () => void {
	let anchorX = 0;
	let anchorY = 0;
	let dragging = false;
	let activated = false;
	let ctrlHeld = false;
	let baseSelection = new Set<string>();
	let marqueeDiv: HTMLDivElement | null = null;
	let rafId = 0;
	let scrollSpeed = 0;
	let lastClientX = 0;
	let lastClientY = 0;

	function onPointerDown(e: PointerEvent) {
		if (e.button !== 0) return;
		if (e.pointerType === "touch") return;
		if (isOnInteractive(e.target)) return;

		const pos = toContentCoords(e, scrollEl);
		anchorX = pos.x;
		anchorY = pos.y;
		lastClientX = e.clientX;
		lastClientY = e.clientY;
		dragging = true;
		activated = false;
		ctrlHeld = e.ctrlKey || e.metaKey;
		baseSelection = ctrlHeld ? new Set(selection.items) : new Set();

		document.addEventListener("pointermove", onPointerMove);
		document.addEventListener("pointerup", onPointerUp);
		document.addEventListener("pointercancel", onPointerUp);
	}

	function buildMarqueeRect(cx: number, cy: number): Rect {
		return {
			x: Math.min(anchorX, cx),
			y: Math.min(anchorY, cy),
			w: Math.abs(cx - anchorX),
			h: Math.abs(cy - anchorY),
		};
	}

	function updateSelection(marquee: Rect) {
		const layout = params.getLayout();
		const items = params.getItems();
		const indices =
			layout.mode === "grid"
				? hitTestGrid(marquee, layout, items.length)
				: hitTestList(marquee, layout, items.length);

		const paths = new Set<string>();
		if (ctrlHeld) {
			for (const p of baseSelection) paths.add(p);
		}
		for (const idx of indices) {
			const item = items[idx];
			if (item && item.name !== "..") {
				paths.add(item.path);
			}
		}
		selection.setExact(paths);
	}

	function updateMarquee() {
		const pos = toContentCoords(
			{ clientX: lastClientX, clientY: lastClientY } as PointerEvent,
			scrollEl,
		);
		const marquee = buildMarqueeRect(pos.x, pos.y);

		if (marqueeDiv) {
			marqueeDiv.style.left = `${marquee.x}px`;
			marqueeDiv.style.top = `${marquee.y}px`;
			marqueeDiv.style.width = `${marquee.w}px`;
			marqueeDiv.style.height = `${marquee.h}px`;
		}

		updateSelection(marquee);
	}

	function tick() {
		if (!dragging) return;
		if (scrollSpeed !== 0) {
			scrollEl.scrollTop += scrollSpeed;
			updateMarquee();
		}
		rafId = requestAnimationFrame(tick);
	}

	function computeScrollSpeed(clientY: number) {
		const rect = scrollEl.getBoundingClientRect();
		const distFromTop = clientY - rect.top;
		const distFromBottom = rect.bottom - clientY;

		if (distFromTop < EDGE_ZONE && distFromTop >= 0) {
			scrollSpeed = -MAX_SCROLL_SPEED * (1 - distFromTop / EDGE_ZONE);
		} else if (distFromBottom < EDGE_ZONE && distFromBottom >= 0) {
			scrollSpeed = MAX_SCROLL_SPEED * (1 - distFromBottom / EDGE_ZONE);
		} else {
			scrollSpeed = 0;
		}
	}

	function onPointerMove(e: PointerEvent) {
		if (!dragging) return;
		e.preventDefault();

		lastClientX = e.clientX;
		lastClientY = e.clientY;

		const pos = toContentCoords(e, scrollEl);

		if (!activated) {
			const dist = Math.hypot(pos.x - anchorX, pos.y - anchorY);
			if (dist < DEAD_ZONE) return;
			activated = true;
			scrollEl.setPointerCapture(e.pointerId);

			const sizerEl = scrollEl.firstElementChild as HTMLElement;
			if (sizerEl) {
				marqueeDiv = document.createElement("div");
				marqueeDiv.style.position = "absolute";
				marqueeDiv.style.pointerEvents = "none";
				marqueeDiv.style.zIndex = "10";
				marqueeDiv.style.backgroundColor = "color-mix(in oklch, var(--primary), transparent 85%)";
				marqueeDiv.style.border = "1px solid color-mix(in oklch, var(--primary), transparent 50%)";
				marqueeDiv.style.borderRadius = "2px";
				sizerEl.appendChild(marqueeDiv);
			}

			rafId = requestAnimationFrame(tick);
		}

		computeScrollSpeed(e.clientY);
		updateMarquee();
	}

	function suppressClick(e: MouseEvent) {
		e.stopPropagation();
	}

	function onPointerUp(e: PointerEvent) {
		const wasActivated = activated;
		cleanup();

		if (wasActivated) {
			scrollEl.addEventListener("click", suppressClick, { capture: true, once: true });
		}
	}

	function cleanup() {
		dragging = false;
		activated = false;
		scrollSpeed = 0;
		cancelAnimationFrame(rafId);

		if (marqueeDiv) {
			marqueeDiv.remove();
			marqueeDiv = null;
		}

		document.removeEventListener("pointermove", onPointerMove);
		document.removeEventListener("pointerup", onPointerUp);
		document.removeEventListener("pointercancel", onPointerUp);
	}

	scrollEl.addEventListener("pointerdown", onPointerDown);

	return () => {
		cleanup();
		scrollEl.removeEventListener("pointerdown", onPointerDown);
	};
}
