import { getSettings } from "$lib/api/settings.js";

let enabled = $state(true);

export const trashEnabled = {
	get enabled() { return enabled; },

	set(value: boolean) {
		enabled = value;
	},

	async refresh() {
		try {
			const s = await getSettings();
			enabled = s["trash.enabled"] !== "false";
		} catch {
			// ignore
		}
	},
};
