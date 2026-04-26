import { request } from "$lib/api";
import type { FileVersion } from "$lib/types";

export async function listVersions(path: string): Promise<{ items: FileVersion[] }> {
	const qs = new URLSearchParams({ path });
	return request<{ items: FileVersion[] }>("GET", `/api/versions?${qs}`);
}

export async function restoreVersion(id: number): Promise<{ status: string }> {
	return request<{ status: string }>("POST", `/api/versions/${id}/restore`);
}

export async function deleteVersion(id: number): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", `/api/versions/${id}`);
}
