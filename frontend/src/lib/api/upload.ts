import { request } from "$lib/api";
import { inBatches } from "$lib/api/files";

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
	const conflicts = await inBatches(paths, async (chunk) =>
		(await request<{ conflicts: ConflictInfo[] }>("POST", "/api/files/check-conflicts", {
			targetDir,
			paths: chunk,
		})).conflicts ?? [],
	);
	return { conflicts };
}
