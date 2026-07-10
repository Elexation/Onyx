import { trashCount as fetchTrashCount } from "$lib/api/trash.js";
import { changes } from "$lib/changes";

let count = $state(0);
let unsub: (() => void) | null = null;

export const trashCount = {
	get count() { return count; },

	set(value: number) {
		count = value;
	},

	async refresh() {
		try {
			const res = await fetchTrashCount();
			count = res.count;
		} catch {
			// ignore
		}
	},

	startPolling() {
		if (unsub) return;
		this.refresh();
		unsub = changes.on("trash.changed", () => this.refresh());
	},

	stopPolling() {
		if (unsub) {
			unsub();
			unsub = null;
		}
	},
};
