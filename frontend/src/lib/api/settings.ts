import { request } from "$lib/api";

export type SettingsMeta = {
	envOverrides: Record<string, string>;
	activeListenPort: string;
};

export type SettingsResponse = {
	values: Record<string, string>;
	meta: SettingsMeta;
};

// In-flight de-dupe: concurrent callers share one round trip; cleared on settle
// so PATCH-followed-by-GET still observes fresh values.
let inflightGet: Promise<SettingsResponse> | null = null;

export async function getSettings(): Promise<SettingsResponse> {
	if (inflightGet) return inflightGet;
	inflightGet = (async () => {
		try {
			return await request<SettingsResponse>("GET", "/api/settings");
		} finally {
			inflightGet = null;
		}
	})();
	return inflightGet;
}

export async function updateSettings(updates: Record<string, string>): Promise<{
	saved: string[];
	errors: Record<string, string>;
}> {
	return request("PATCH", "/api/settings", updates);
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
	await request("POST", "/api/auth/change-password", { currentPassword, newPassword });
}
