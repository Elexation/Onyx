import { goto } from "$app/navigation";
import { request, setCsrfToken } from "./api.js";
import type { AuthStatus } from "./types.js";

export const auth = $state({
	checked: false,
	authenticated: false,
	firstRun: false,
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

export async function logout(): Promise<void> {
	await request("POST", "/api/auth/logout");
	auth.authenticated = false;
	setCsrfToken("");
	await goto("/login");
}
