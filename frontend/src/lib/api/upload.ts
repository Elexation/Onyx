import { request } from "$lib/api";

export type ConflictInfo = {
	path: string;
	isDir: boolean;
	size: number;
	modTime: number;
};

export async function checkConflicts(
	targetDir: string,
	paths: string[],
): Promise<{ conflicts: ConflictInfo[] }> {
	return request<{ conflicts: ConflictInfo[] }>("POST", "/api/files/check-conflicts", {
		targetDir,
		paths,
	});
}
