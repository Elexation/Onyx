import { request } from "$lib/api.js";
import type { ShareLink } from "$lib/types.js";

export interface CreateShareRequest {
	path: string;
	isDir: boolean;
	expiresIn?: string;
	password?: string;
}

export async function createShare(req: CreateShareRequest): Promise<ShareLink> {
	return request<ShareLink>("POST", "/api/shares", req);
}

export async function listShares(): Promise<{ shares: ShareLink[] }> {
	return request<{ shares: ShareLink[] }>("GET", "/api/shares");
}

export async function getShareByPath(path: string): Promise<ShareLink | null> {
	const res = await request<{ share: ShareLink | null }>("GET", `/api/shares/by-path?path=${encodeURIComponent(path)}`);
	return res.share;
}

export async function deleteShare(id: number): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", `/api/shares/${id}`);
}
