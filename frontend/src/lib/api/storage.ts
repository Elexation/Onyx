import { request } from "$lib/api";

export type StorageUsage = { used: number; total: number };

export async function getStorageUsage(): Promise<StorageUsage> {
	return request<StorageUsage>("GET", "/api/storage");
}
