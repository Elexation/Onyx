import type { Action } from "svelte/action";

export const longpress: Action<HTMLElement, () => void> = (node, callback) => {
	let currentCallback = callback;
	let timer: ReturnType<typeof setTimeout> | null = null;
	let fired = false;

	function start(e: PointerEvent) {
		// Touch and pen only: a still mouse hold would toggle selection and eat the click
		if (e.pointerType === "mouse") return;
		fired = false;
		timer = setTimeout(() => {
			fired = true;
			currentCallback();
		}, 300);
	}

	function cancel() {
		if (timer) {
			clearTimeout(timer);
			timer = null;
		}
	}

	function preventClick(e: MouseEvent) {
		if (fired) {
			e.preventDefault();
			e.stopImmediatePropagation();
			fired = false;
		}
	}

	node.addEventListener("pointerdown", start);
	node.addEventListener("pointerup", cancel);
	node.addEventListener("pointercancel", cancel);
	node.addEventListener("pointermove", cancel);
	node.addEventListener("click", preventClick, true);

	return {
		update(newCallback: () => void) {
			currentCallback = newCallback;
		},
		destroy() {
			cancel();
			node.removeEventListener("pointerdown", start);
			node.removeEventListener("pointerup", cancel);
			node.removeEventListener("pointercancel", cancel);
			node.removeEventListener("pointermove", cancel);
			node.removeEventListener("click", preventClick, true);
		},
	};
};
