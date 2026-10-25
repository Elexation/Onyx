import { request, setCsrfToken } from "$lib/api";

export type SettingsMeta = {
	envOverrides: Record<string, string>;
	activeListenPort: string;
	activeTLS: boolean;
};

export type SettingsResponse = {
	values: Record<string, string>;
	meta: SettingsMeta;
};

// In-flight de-dupe: concurrent callers share one round trip. A PATCH drops the
// shared GET, since one started before the PATCH committed returns old values.
let inflightGet: Promise<SettingsResponse> | null = null;

export async function getSettings(): Promise<SettingsResponse> {
	if (inflightGet) return inflightGet;
	const p: Promise<SettingsResponse> = request<SettingsResponse>("GET", "/api/settings").finally(() => {
		if (inflightGet === p) inflightGet = null;
	});
	inflightGet = p;
	return p;
}

export async function updateSettings(updates: Record<string, string>): Promise<{
	saved: string[];
	errors: Record<string, string>;
}> {
	try {
		return await request("PATCH", "/api/settings", updates);
	} finally {
		inflightGet = null;
	}
}

// The server rotates the session, so adopt its new CSRF token or every later write 403s
export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
	const res = await request<{ csrfToken?: string }>("POST", "/api/auth/change-password", { currentPassword, newPassword });
	if (res?.csrfToken) setCsrfToken(res.csrfToken);
}
