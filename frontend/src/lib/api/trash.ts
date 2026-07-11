import { request } from "$lib/api";
import type { TrashItem } from "$lib/types";

export type RestoreStrategy = "replace" | "keepBoth" | "skip";

export type RestoreConflictMeta = { size: number; modTime: number };

export type RestoreConflictItem = {
	id: string;
	path: string;
	isDir: boolean;
	existing: RestoreConflictMeta;
	restoring: RestoreConflictMeta;
};

export async function listTrash(): Promise<{ items: TrashItem[]; count: number }> {
	return request<{ items: TrashItem[]; count: number }>("GET", "/api/trash");
}

export async function trashCount(): Promise<{ count: number }> {
	return request<{ count: number }>("GET", "/api/trash/count");
}

export async function checkRestoreConflicts(
	ids: string[],
): Promise<{ conflicts: RestoreConflictItem[] }> {
	return request<{ conflicts: RestoreConflictItem[] }>(
		"POST",
		"/api/trash/check-restore-conflicts",
		{ ids },
	);
}

export async function restoreTrashItem(
	id: string,
	strategy?: RestoreStrategy,
): Promise<{ status: string; path?: string }> {
	const qs = strategy ? `?strategy=${strategy}` : "";
	return request<{ status: string; path?: string }>("POST", `/api/trash/${id}/restore${qs}`);
}

export async function permanentDeleteTrashItem(id: string): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", `/api/trash/${id}`);
}

export async function emptyTrash(): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", "/api/trash");
}
