import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export type WithElementRef<T, E extends HTMLElement = HTMLElement> = T & { ref?: E | null };

// Escapes each path segment for use in a URL. Preserves `/` separators while
// encoding `#`, `?`, `%`, spaces, and other reserved characters that would
// otherwise break routing or become fragments/query strings.
export function encodeFilePath(path: string): string {
	return path.split("/").map(encodeURIComponent).join("/");
}
