export interface AuthStatus {
	firstRun: boolean;
	authenticated: boolean;
	csrfToken?: string;
}

export interface ApiError {
	error: string;
}

export interface FileInfo {
	name: string;
	path: string;
	isDir: boolean;
	size: number;
	modTime: number;
	mimeType?: string;
}

export interface DirectoryListing {
	path: string;
	items: FileInfo[];
}

export interface TrashItem {
	id: string;
	originalPath: string;
	deletedAt: number;
	size: number;
	isDir: boolean;
}

export interface FileVersion {
	id: number;
	filePath: string;
	createdAt: number;
	size: number;
}

export interface SearchResult {
	name: string;
	path: string;
	isDir: boolean;
}

export interface ShareLink {
	id: number;
	token?: string;
	tokenLast8: string;
	filePath: string;
	isDir: boolean;
	createdAt: number;
	expiresAt?: number;
	hasPassword: boolean;
	downloadCount: number;
}
