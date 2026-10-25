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

// Focusable controls that act on Enter themselves (buttons, links, rows, cards)
const OWN_ENTER = 'button, a[href], [role="button"], [role="link"], [role="menuitem"], [role="option"], [tabindex]';

// A focused inner control owns Enter; taking it here would cancel its click
function ownsEnter(node: HTMLElement, target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement)) return false;
	const control = target.closest(OWN_ENTER);
	return !!control && control !== node && node.contains(control);
}

export const shortcuts: Action<HTMLElement, ShortcutMap> = (node, map) => {
	let currentMap = map;

	function handler(e: KeyboardEvent) {
		if (isInputElement(e.target)) return;
		const combo = toCombo(e);
		const fn = currentMap[combo];
		if (!fn) return;
		if (combo === "enter" && ownsEnter(node, e.target)) return;
		e.preventDefault();
		e.stopPropagation();
		fn(e);
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
