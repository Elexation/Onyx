import { listShares } from "$lib/api/shares.js";
import { changes } from "$lib/changes";

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
			// ignore, keep prior cache
		}
	},
};

// Cross-tab sync: refresh the cache when any tab/device creates or revokes
// a share. Module-load subscription is safe because changes.on() just
// registers the callback; events flow only after changes.start() runs from
// (app)/+layout.svelte. Cost is one always-on listener for the SPA lifetime.
//
// Debounced: a poll batch with N share events would otherwise fire N full
// listShares() round trips. Trailing-edge 250ms coalesces bursts to one.
let refreshDebounceTimer: ReturnType<typeof setTimeout> | null = null;
changes.on("share.changed", () => {
	if (refreshDebounceTimer) clearTimeout(refreshDebounceTimer);
	refreshDebounceTimer = setTimeout(() => {
		refreshDebounceTimer = null;
		sharedPaths.refresh();
	}, 250);
});
