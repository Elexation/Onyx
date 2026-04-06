import { goto } from "$app/navigation";

let csrfToken = "";

export function setCsrfToken(token: string) {
	csrfToken = token;
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
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

	if (res.status === 401 && !path.includes("/api/auth/")) {
		await goto("/login");
		throw new Error("unauthorized");
	}

	if (!res.ok) {
		const err = await res.json().catch(() => ({ error: "request failed" }));
		throw new Error(err.error || "request failed");
	}

	return res.json() as Promise<T>;
}
