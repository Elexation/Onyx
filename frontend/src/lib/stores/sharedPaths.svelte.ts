import { listShares } from "$lib/api/shares.js";

let paths = $state<Set<string>>(new Set());

export const sharedPaths = {
	has(path: string): boolean {
		return paths.has(path);
	},

	add(path: string) {
		const next = new Set(paths);
		next.add(path);
		paths = next;
	},

	remove(path: string) {
		if (!paths.has(path)) return;
		const next = new Set(paths);
		next.delete(path);
		paths = next;
	},

	async refresh() {
		try {
			const res = await listShares();
			paths = new Set(res.shares.map((s) => s.filePath));
		} catch {
			// ignore — keep prior cache
		}
	},
};
