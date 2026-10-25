import { goto } from "$app/navigation";
import { toast } from "svelte-sonner";
import { request, setCsrfToken, setUnauthorizedHandler } from "./api.js";
import type { AuthStatus } from "./types.js";

export const auth = $state({
	checked: false,
	authenticated: false,
	firstRun: false,
});

setUnauthorizedHandler(() => {
	auth.authenticated = false;
	setCsrfToken("");
});

export async function checkStatus(): Promise<void> {
	const status = await request<AuthStatus>("GET", "/api/auth/status");
	auth.firstRun = status.firstRun;
	auth.authenticated = status.authenticated;
	auth.checked = true;
	if (status.csrfToken) {
		setCsrfToken(status.csrfToken);
	}
}

export async function login(password: string): Promise<void> {
	const res = await request<AuthStatus>("POST", "/api/auth/login", { password });
	auth.authenticated = true;
	auth.firstRun = false;
	if (res.csrfToken) {
		setCsrfToken(res.csrfToken);
	}
}

export async function setup(password: string): Promise<void> {
	const res = await request<AuthStatus>("POST", "/api/auth/setup", { password });
	auth.authenticated = true;
	auth.firstRun = false;
	if (res.csrfToken) {
		setCsrfToken(res.csrfToken);
	}
}

// Stays signed in on failure: the session cookie is HttpOnly, so only the server can end it
export async function logout(): Promise<void> {
	try {
		await request("POST", "/api/auth/logout");
	} catch (e) {
		// 401 means the session already ended
		if ((e as { status?: number }).status !== 401) {
			toast.error("Couldn't sign out. Check your connection and try again");
			return;
		}
	}
	auth.authenticated = false;
	setCsrfToken("");
	await goto("/login", { replaceState: true });
}
