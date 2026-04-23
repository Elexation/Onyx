import { trashCount as fetchTrashCount } from "$lib/api/trash.js";

let count = $state(0);
let polling: ReturnType<typeof setInterval> | null = null;

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
		this.refresh();
		polling = setInterval(() => this.refresh(), 30_000);
	},

	stopPolling() {
		if (polling) {
			clearInterval(polling);
			polling = null;
		}
	},
};
