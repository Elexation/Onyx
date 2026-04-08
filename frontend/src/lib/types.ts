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
