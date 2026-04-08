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
