import { request } from "$lib/api.js";
import type { PersonalAccessToken, TokenScope } from "$lib/types.js";

export interface CreateTokenRequest {
	name: string;
	scope: TokenScope;
	expiresAt: number | null;
}

export interface TokenListResponse {
	tokens: PersonalAccessToken[];
	max: number;
}

export async function createToken(req: CreateTokenRequest): Promise<PersonalAccessToken> {
	return request<PersonalAccessToken>("POST", "/api/tokens", req);
}

export async function listTokens(): Promise<TokenListResponse> {
	return request<TokenListResponse>("GET", "/api/tokens");
}

export async function revokeToken(id: number): Promise<{ status: string }> {
	return request<{ status: string }>("DELETE", `/api/tokens/${id}`);
}
