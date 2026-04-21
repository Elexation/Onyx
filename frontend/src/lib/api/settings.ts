import { request } from "$lib/api";

export async function getSettings(): Promise<Record<string, string>> {
	return request<Record<string, string>>("GET", "/api/settings");
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
