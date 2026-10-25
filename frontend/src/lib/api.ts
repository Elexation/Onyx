import { goto } from "$app/navigation";

let csrfToken = "";
let onUnauthorized: (() => void) | null = null;

export function setCsrfToken(token: string) {
	csrfToken = token;
}

export function getCsrfToken(): string {
	return csrfToken;
}

// auth.svelte.ts registers here (it imports this module, so no direct import back)
export function setUnauthorizedHandler(fn: () => void) {
	onUnauthorized = fn;
}

// Re-read the session's CSRF token, e.g. after another tab logged in again. False if none.
async function refreshCsrfToken(): Promise<boolean> {
	try {
		const res = await fetch("/api/auth/status", { credentials: "include" });
		if (!res.ok) return false;
		const status = (await res.json()) as { csrfToken?: string };
		if (!status.csrfToken || status.csrfToken === csrfToken) return false;
		csrfToken = status.csrfToken;
		return true;
	} catch {
		return false;
	}
}

export async function request<T>(method: string, path: string, body?: unknown, retried = false): Promise<T> {
	const opts: RequestInit = {
		method,
		credentials: "include",
		headers: {} as Record<string, string>,
	};

	if (body !== undefined) {
		(opts.headers as Record<string, string>)["Content-Type"] = "application/json";
		opts.body = JSON.stringify(body);
	}

	if (method !== "GET" && method !== "HEAD" && csrfToken) {
		(opts.headers as Record<string, string>)["X-CSRF-Token"] = csrfToken;
	}

	const res = await fetch(path, opts);

	if (res.status === 401 && !path.startsWith("/api/auth/")) {
		// Clear auth first, or the root layout bounces /login straight back to /files
		onUnauthorized?.();
		await goto("/login");
		throw new Error("unauthorized");
	}

	if (!res.ok) {
		const err = await res.json().catch(() => ({ error: "request failed" }));
		// The session rotated under this tab (password change, re-login elsewhere): refresh once
		if (res.status === 403 && err.error === "invalid csrf token" && !retried && (await refreshCsrfToken())) {
			return request<T>(method, path, body, true);
		}
		const e = new Error(err.error || "request failed") as Error & { status: number };
		e.status = res.status;
		throw e;
	}

	if (res.status === 204) return undefined as T;
	return res.json() as Promise<T>;
}
