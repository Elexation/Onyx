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

// In-flight de-dupe: concurrent callers (e.g. /shares page + sharedPaths
// store both reacting to share.changed) share one round trip; cleared on
// settle so a follow-up call after a mutation observes fresh values.
let inflightList: Promise<{ shares: ShareLink[] }> | null = null;

export async function listShares(): Promise<{ shares: ShareLink[] }> {
	if (inflightList) return inflightList;
	inflightList = (async () => {
		try {
			return await request<{ shares: ShareLink[] }>("GET", "/api/shares");
		} finally {
			inflightList = null;
		}
	})();
	return inflightList;
}

export async function getShareByPath(path: string): Promise<ShareLink | null> {
	const res = await request<{ share: ShareLink | null }>("GET", `/api/shares/by-path?path=${encodeURIComponent(path)}`);
	return res.share;
}

export async function deleteShare(id: number): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", `/api/shares/${id}`);
}

export async function shareCount(): Promise<{ count: number }> {
	return request<{ count: number }>("GET", "/api/shares/count");
}
