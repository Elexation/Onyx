import type { Action } from "svelte/action";

export type ShortcutMap = Record<string, (e: KeyboardEvent) => void>;

function isInputElement(target: EventTarget | null): boolean {
	if (!target || !(target instanceof HTMLElement)) return false;
	const tag = target.tagName;
	return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable;
}

function toCombo(e: KeyboardEvent): string {
	const parts: string[] = [];
	if (e.ctrlKey || e.metaKey) parts.push("ctrl");
	if (e.shiftKey) parts.push("shift");
	if (e.altKey) parts.push("alt");
	parts.push(e.key.toLowerCase());
	return parts.join("+");
}

export const shortcuts: Action<HTMLElement, ShortcutMap> = (node, map) => {
	let currentMap = map;

	function handler(e: KeyboardEvent) {
		if (isInputElement(e.target)) return;
		const combo = toCombo(e);
		const fn = currentMap[combo];
		if (fn) {
			e.preventDefault();
			e.stopPropagation();
			fn(e);
		}
	}

	node.addEventListener("keydown", handler);

	return {
		update(newMap: ShortcutMap) {
			currentMap = newMap;
		},
		destroy() {
			node.removeEventListener("keydown", handler);
		},
	};
};
