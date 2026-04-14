import { move, copy } from "$lib/api/files.js";
import type { BatchResult } from "$lib/api/files.js";

export type ClipboardMode = "copy" | "cut";

let paths = $state<string[]>([]);
let mode = $state<ClipboardMode>("copy");

export const clipboard = {
	get paths() { return paths; },
	get mode() { return mode; },
	get hasItems() { return paths.length > 0; },

	isCut(path: string): boolean {
		return mode === "cut" && paths.includes(path);
	},

	copy(items: string[]) {
		paths = [...items];
		mode = "copy";
	},

	cut(items: string[]) {
		paths = [...items];
		mode = "cut";
	},

	async paste(destination: string): Promise<BatchResult[]> {
		if (paths.length === 0) return [];
		const fn = mode === "cut" ? move : copy;
		const res = await fn(paths, destination);
		paths = [];
		return res.results;
	},

	clear() {
		paths = [];
	},
};
