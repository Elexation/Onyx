import type { Action } from "svelte/action";
import { DRAG_MIME } from "./draggable.js";

interface DroppableParams {
	/** The directory path this drop target represents */
	path: string;
	/** Callback when items are dropped: (sourcePaths, destinationPath) */
	ondrop: (paths: string[], destination: string) => void;
	/** Whether this drop target is active (default true) */
	enabled?: boolean;
}

function isInternalDrag(e: DragEvent): boolean {
	return e.dataTransfer?.types.includes(DRAG_MIME) ?? false;
}

export const droppable: Action<HTMLElement, DroppableParams> = (node, params) => {
	let current = params;
	let dragOverCount = 0;

	function onDragEnter(e: DragEvent) {
		if (current.enabled === false || !isInternalDrag(e)) return;
		e.preventDefault();
		dragOverCount++;
		if (dragOverCount === 1) {
			node.classList.add("drop-target-active");
		}
	}

	function onDragOver(e: DragEvent) {
		if (current.enabled === false || !isInternalDrag(e)) return;
		e.preventDefault();
		if (e.dataTransfer) {
			e.dataTransfer.dropEffect = "move";
		}
	}

	function onDragLeave(e: DragEvent) {
		if (current.enabled === false || !isInternalDrag(e)) return;
		dragOverCount--;
		if (dragOverCount <= 0) {
			dragOverCount = 0;
			node.classList.remove("drop-target-active");
		}
	}

	function onDrop(e: DragEvent) {
		if (current.enabled === false || !isInternalDrag(e)) return;
		e.preventDefault();
		e.stopPropagation();
		dragOverCount = 0;
		node.classList.remove("drop-target-active");

		const raw = e.dataTransfer?.getData(DRAG_MIME);
		if (!raw) return;

		try {
			const paths: string[] = JSON.parse(raw);
			if (paths.length > 0) {
				// Don't drop onto yourself or your own parent
				const dest = current.path;
				const valid = paths.filter((p) => {
					// Can't move a folder into itself
					if (p === dest) return false;
					// Can't move a folder into its own subtree
					if (dest.startsWith(p + "/")) return false;
					// Can't move into the same parent (no-op)
					const parentOfSource = p.includes("/") ? p.slice(0, p.lastIndexOf("/")) : "";
					if (parentOfSource === dest) return false;
					return true;
				});
				if (valid.length > 0) {
					current.ondrop(valid, dest);
				}
			}
		} catch {
			// Invalid data, ignore
		}
	}

	node.addEventListener("dragenter", onDragEnter);
	node.addEventListener("dragover", onDragOver);
	node.addEventListener("dragleave", onDragLeave);
	node.addEventListener("drop", onDrop);

	return {
		update(newParams: DroppableParams) {
			current = newParams;
		},
		destroy() {
			node.classList.remove("drop-target-active");
			node.removeEventListener("dragenter", onDragEnter);
			node.removeEventListener("dragover", onDragOver);
			node.removeEventListener("dragleave", onDragLeave);
			node.removeEventListener("drop", onDrop);
		},
	};
};
