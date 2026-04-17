import { request } from "$lib/api";

export async function checkConflicts(
	targetDir: string,
	paths: string[],
): Promise<{ conflicts: string[] }> {
	return request<{ conflicts: string[] }>("POST", "/api/files/check-conflicts", {
		targetDir,
		paths,
	});
}
