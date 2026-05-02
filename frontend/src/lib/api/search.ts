import { request } from "$lib/api";
import type { SearchResult } from "$lib/types";

export interface SearchResponse {
	results: SearchResult[];
	total: number;
}

export async function search(query: string): Promise<SearchResponse> {
	return request<SearchResponse>("GET", `/api/search?q=${encodeURIComponent(query)}`);
}
