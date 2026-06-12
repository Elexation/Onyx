import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export type WithElementRef<T, E extends HTMLElement = HTMLElement> = T & { ref?: E | null };

export type WithoutChild<T> = T extends { child?: any } ? Omit<T, "child"> : T;
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, "children"> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;

// Escapes each path segment for use in a URL. Preserves `/` separators while
// encoding `#`, `?`, `%`, spaces, and other reserved characters that would
// otherwise break routing or become fragments/query strings.
export function encodeFilePath(path: string): string {
	return path.split("/").map(encodeURIComponent).join("/");
}
