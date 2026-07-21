export type Section = { id: string; label: string; keywords?: string };
export type ScopeGroup = { label: string; scope: "read" | "upload" | "full"; sections: Section[] };

export const overview: Section[] = [
	{ id: "authentication", label: "Authentication", keywords: "auth bearer token header" },
	{ id: "scopes", label: "Scopes", keywords: "read upload full permissions" },
	{ id: "errors", label: "Errors & Conventions", keywords: "error timestamp unix path" },
];

export const groups: ScopeGroup[] = [
	{
		label: "Read",
		scope: "read",
		sections: [
			{ id: "files", label: "Files", keywords: "GET list directory file info metadata" },
			{ id: "download", label: "Download", keywords: "GET zip archive attachment" },
			{ id: "preview", label: "Preview", keywords: "GET inline serve" },
			{ id: "search", label: "Search", keywords: "GET query fulltext filename" },
			{ id: "thumbnails", label: "Thumbnails", keywords: "GET thumb image jpeg size" },
			{ id: "streaming", label: "Streaming", keywords: "GET HLS video stream master playlist segment" },
			{ id: "shares-read", label: "Shares", keywords: "GET list share link count by-path" },
			{ id: "trash-read", label: "Trash", keywords: "GET list trash count deleted" },
			{ id: "versions-read", label: "Versions", keywords: "GET list version history" },
			{ id: "storage", label: "Storage", keywords: "GET disk usage space" },
		],
	},
	{
		label: "Upload",
		scope: "upload",
		sections: [
			{ id: "check-conflicts", label: "Check Conflicts", keywords: "POST conflict upload exists" },
			{ id: "mkdir", label: "Make Directory", keywords: "POST mkdir create folder" },
			{ id: "upload", label: "Upload (tus)", keywords: "POST PATCH tus resumable file upload" },
		],
	},
	{
		label: "Full",
		scope: "full",
		sections: [
			{ id: "rename", label: "Rename", keywords: "POST rename file name" },
			{ id: "move", label: "Move", keywords: "POST move destination" },
			{ id: "copy", label: "Copy", keywords: "POST copy duplicate" },
			{ id: "delete", label: "Delete", keywords: "DELETE remove permanent trash" },
			{ id: "shares-write", label: "Create Share", keywords: "POST share link create password expiry" },
			{ id: "shares-delete", label: "Delete Share", keywords: "DELETE revoke share" },
			{ id: "trash-write", label: "Trash Operations", keywords: "POST DELETE restore purge empty" },
			{ id: "versions-write", label: "Version Operations", keywords: "POST DELETE restore version" },
		],
	},
];

export const allSections: Section[] = [...overview, ...groups.flatMap((g) => g.sections)];

export const scopeColors = {
	read: "bg-muted text-muted-foreground",
	upload: "bg-accent-brand-dim text-accent-brand",
	full: "bg-destructive/15 text-destructive",
} as const;

export function matchesFilter(s: Section, query: string): boolean {
	const q = query.trim().toLowerCase();
	if (!q) return true;
	return s.label.toLowerCase().includes(q) || (s.keywords?.toLowerCase().includes(q) ?? false);
}
