import type { Action } from "svelte/action";
import { selection } from "$lib/stores/selection.svelte.js";

export const DRAG_MIME = "application/x-onyx-paths";

interface DraggableParams {
	path: string;
	isDir?: boolean;
}

export const draggable: Action<HTMLElement, DraggableParams> = (node, params) => {
	let current = params;

	function onDragStart(e: DragEvent) {
		if (!e.dataTransfer) return;

		// If the dragged item is selected, drag all selected items; otherwise just this one
		const paths = selection.has(current.path)
			? [...selection.items]
			: [current.path];

		e.dataTransfer.effectAllowed = "move";
		e.dataTransfer.setData(DRAG_MIME, JSON.stringify(paths));
		// Also set text/plain so dataTransfer.types is non-empty for dragover checks
		e.dataTransfer.setData("text/plain", "");

		// Badge showing count when dragging multiple items
		if (paths.length > 1) {
			const badge = document.createElement("div");
			badge.textContent = `${paths.length} items`;
			Object.assign(badge.style, {
				position: "absolute",
				top: "-9999px",
				left: "-9999px",
				padding: "4px 10px",
				borderRadius: "6px",
				background: "hsl(var(--primary))",
				color: "hsl(var(--primary-foreground))",
				fontSize: "12px",
				fontWeight: "500",
				whiteSpace: "nowrap",
			});
			document.body.appendChild(badge);
			e.dataTransfer.setDragImage(badge, badge.offsetWidth / 2, badge.offsetHeight / 2);
			// Clean up after the browser has captured the image
			requestAnimationFrame(() => badge.remove());
		}

		// Add a class to the body so other components can detect an internal drag
		document.body.classList.add("onyx-internal-drag");
	}

	function onDragEnd() {
		document.body.classList.remove("onyx-internal-drag");
	}

	node.draggable = true;
	node.addEventListener("dragstart", onDragStart);
	node.addEventListener("dragend", onDragEnd);

	return {
		update(newParams: DraggableParams) {
			current = newParams;
		},
		destroy() {
			node.removeEventListener("dragstart", onDragStart);
			node.removeEventListener("dragend", onDragEnd);
			node.draggable = false;
		},
	};
};
