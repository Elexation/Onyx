import { request } from "$lib/api";

export type SettingsMeta = {
	envOverrides: Record<string, string>;
	activeListenPort: string;
};

export type SettingsResponse = {
	values: Record<string, string>;
	meta: SettingsMeta;
};

export async function getSettings(): Promise<SettingsResponse> {
	return request<SettingsResponse>("GET", "/api/settings");
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
