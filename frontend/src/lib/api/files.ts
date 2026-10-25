import { request } from "$lib/api";
import { encodeFilePath } from "$lib/utils";
import type { DirectoryListing } from "$lib/types";

export async function listDirectory(path: string, opts?: { dirsOnly?: boolean }): Promise<DirectoryListing> {
	const trimmed = path.replace(/^\/+/, "");
	const normalized = trimmed ? `/${encodeFilePath(trimmed)}` : "/";
	const params = new URLSearchParams({ showHidden: "true" });
	if (opts?.dirsOnly) params.set("dirsOnly", "true");
	return request<DirectoryListing>("GET", `/api/files${normalized}?${params}`);
}

export function getDownloadUrl(path: string): string {
	return `/api/download${encodeFilePath(path)}`;
}

export function getZipDownloadUrl(paths: string[]): string {
	const params = new URLSearchParams();
	for (const p of paths) {
		params.append("path", p);
	}
	return `/api/download/zip?${params.toString()}`;
}

export interface BatchResult {
	path: string;
	success: boolean;
	error?: string;
}

// Keep batches at or under the server's per-request path cap
export const MAX_BATCH_PATHS = 500;

// Run fn over paths in server-sized chunks, one request at a time.
export async function inBatches<T>(paths: string[], fn: (chunk: string[]) => Promise<T[]>): Promise<T[]> {
	const out: T[] = [];
	for (let i = 0; i < paths.length; i += MAX_BATCH_PATHS) {
		out.push(...(await fn(paths.slice(i, i + MAX_BATCH_PATHS))));
	}
	return out;
}

export async function mkdir(path: string): Promise<{ path: string }> {
	return request<{ path: string }>("POST", "/api/files/mkdir", { path });
}

export async function rename(path: string, newName: string): Promise<{ status: string }> {
	return request<{ status: string }>("POST", "/api/files/rename", { path, newName });
}

export async function move(paths: string[], destination: string): Promise<{ results: BatchResult[] }> {
	const results = await inBatches(paths, async (chunk) =>
		(await request<{ results: BatchResult[] }>("POST", "/api/files/move", { paths: chunk, destination })).results ?? [],
	);
	return { results };
}

export async function copy(paths: string[], destination: string): Promise<{ results: BatchResult[] }> {
	const results = await inBatches(paths, async (chunk) =>
		(await request<{ results: BatchResult[] }>("POST", "/api/files/copy", { paths: chunk, destination })).results ?? [],
	);
	return { results };
}

export async function deleteFiles(paths: string[], permanent = false): Promise<{ results: BatchResult[] }> {
	const results = await inBatches(paths, async (chunk) =>
		(await request<{ results: BatchResult[] }>("DELETE", "/api/files", { paths: chunk, permanent })).results ?? [],
	);
	return { results };
}
