export interface AuthStatus {
	firstRun: boolean;
	authenticated: boolean;
	csrfToken?: string;
}

export interface ApiError {
	error: string;
}
