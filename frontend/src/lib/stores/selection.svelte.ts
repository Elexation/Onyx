let selected = $state<Set<string>>(new Set());
let lastSelected = $state<string | null>(null);
let active = $state(false);

export const selection = {
	get items() { return selected; },
	get count() { return selected.size; },
	get isActive() { return active; },
	get lastSelected() { return lastSelected; },

	has(path: string): boolean {
		return selected.has(path);
	},

	select(path: string) {
		selected = new Set([path]);
		lastSelected = path;
		active = true;
	},

	toggle(path: string) {
		const next = new Set(selected);
		if (next.has(path)) {
			next.delete(path);
		} else {
			next.add(path);
		}
		selected = next;
		lastSelected = path;
		active = next.size > 0;
	},

	selectRange(path: string, allPaths: string[]) {
		if (!lastSelected) {
			this.select(path);
			return;
		}
		const start = allPaths.indexOf(lastSelected);
		const end = allPaths.indexOf(path);
		if (start === -1 || end === -1) {
			this.select(path);
			return;
		}
		const lo = Math.min(start, end);
		const hi = Math.max(start, end);
		const next = new Set(selected);
		for (let i = lo; i <= hi; i++) {
			next.add(allPaths[i]);
		}
		selected = next;
		lastSelected = path;
		active = true;
	},

	selectAll(allPaths: string[]) {
		selected = new Set(allPaths);
		active = allPaths.length > 0;
	},

	clear() {
		selected = new Set();
		lastSelected = null;
		active = false;
	},
};
