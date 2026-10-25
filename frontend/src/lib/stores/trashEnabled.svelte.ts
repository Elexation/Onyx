import { getSettings } from "$lib/api/settings.js";
import { changes } from "$lib/changes";
import { parseDurationHours } from "$lib/utils/format.js";

let enabled = $state(true);
// Auto-purge age in hours; 0 = never, null = unknown
let purgeAgeHours = $state<number | null>(null);

export const trashEnabled = {
	get enabled() { return enabled; },
	get purgeAgeHours() { return purgeAgeHours; },

	set(value: boolean) {
		enabled = value;
	},

	async refresh() {
		try {
			const s = await getSettings();
			enabled = s.values["trash.enabled"] !== "false";
			purgeAgeHours = parseDurationHours(s.values["trash.purge_age"] ?? "720h");
		} catch {
			// ignore
		}
	},
};

// A disable on another tab or device must reach DeleteDialog's "can be restored" promise
changes.on("settings.changed", () => {
	trashEnabled.refresh();
});
