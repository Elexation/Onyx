import { request } from "$lib/api";
import type { DirectoryListing } from "$lib/types";

export async function listDirectory(path: string, showHidden = false): Promise<DirectoryListing> {
	const normalized = path ? `/${path}` : "/";
	const params = showHidden ? "?showHidden=true" : "";
	return request<DirectoryListing>("GET", `/api/files${normalized}${params}`);
}

export function getDownloadUrl(path: string): string {
	return `/api/download/${path}`;
}

export interface BatchResult {
	path: string;
	success: boolean;
	error?: string;
}

export async function mkdir(path: string): Promise<{ path: string }> {
	return request<{ path: string }>("POST", "/api/files/mkdir", { path });
}

export async function rename(path: string, newName: string): Promise<{ status: string }> {
	return request<{ status: string }>("POST", "/api/files/rename", { path, newName });
}

export async function move(paths: string[], destination: string): Promise<{ results: BatchResult[] }> {
	return request<{ results: BatchResult[] }>("POST", "/api/files/move", { paths, destination });
}

export async function copy(paths: string[], destination: string): Promise<{ results: BatchResult[] }> {
	return request<{ results: BatchResult[] }>("POST", "/api/files/copy", { paths, destination });
}

export async function deleteFiles(paths: string[]): Promise<{ results: BatchResult[] }> {
	return request<{ results: BatchResult[] }>("DELETE", "/api/files", { paths });
}
