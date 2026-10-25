import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export type WithElementRef<T, E extends HTMLElement = HTMLElement> = T & { ref?: E | null };

export type WithoutChild<T> = T extends { child?: any } ? Omit<T, "child"> : T;
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, "children"> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;

// Encode each path segment, keeping "/" separators
export function encodeFilePath(path: string): string {
	return path.split("/").map(encodeURIComponent).join("/");
}

// SPA route for a folder/file path, segments encoded so "#", "?" and "%" survive
export function filesRoute(path: string): string {
	const trimmed = path.replace(/^\/+/, "");
	return trimmed ? `/files/${encodeFilePath(trimmed)}` : "/files";
}

export function basename(path: string): string {
	return path.split("/").pop() ?? "";
}

export function dirname(path: string): string {
	const i = path.lastIndexOf("/");
	return i <= 0 ? "/" : path.substring(0, i);
}
