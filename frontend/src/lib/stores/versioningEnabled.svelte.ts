import { getSettings } from "$lib/api/settings.js";

let enabled = $state(true);
let maxFileSize = $state(0);

export const versioningEnabled = {
	get enabled() { return enabled; },
	// 0 = no cap. Lets UI copy qualify "older versions are kept" honestly.
	get maxFileSize() { return maxFileSize; },

	set(value: boolean) {
		enabled = value;
	},

	async refresh() {
		try {
			const s = await getSettings();
			enabled = s.values["versions.enabled"] !== "false";
			maxFileSize = Number(s.values["versions.max_file_size"]) || 0;
		} catch {
			// ignore
		}
	},
};
