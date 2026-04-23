import { request } from "$lib/api";
import type { TrashItem } from "$lib/types";

export async function listTrash(): Promise<{ items: TrashItem[]; count: number }> {
	return request<{ items: TrashItem[]; count: number }>("GET", "/api/trash");
}

export async function trashCount(): Promise<{ count: number }> {
	return request<{ count: number }>("GET", "/api/trash/count");
}

export async function restoreTrashItem(id: string): Promise<{ status: string }> {
	return request<{ status: string }>("POST", `/api/trash/${id}/restore`);
}

export async function permanentDeleteTrashItem(id: string): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", `/api/trash/${id}`);
}

export async function emptyTrash(): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", "/api/trash");
}
