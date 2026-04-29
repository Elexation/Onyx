import type { FileInfo } from "$lib/types";

export type PreviewType = "text" | "markdown" | "image" | "video" | "audio" | "pdf";

const TEXT_MIME_PREFIXES = ["text/"];
const TEXT_MIME_EXACT = [
	"application/json",
	"application/xml",
	"application/javascript",
	"application/typescript",
	"application/x-yaml",
	"application/toml",
	"application/x-sh",
	"application/x-httpd-php",
];

const MARKDOWN_EXTENSIONS = [".md", ".markdown", ".mdx"];

const IMAGE_MIME_PREFIX = "image/";
const VIDEO_MIME_PREFIX = "video/";
const AUDIO_MIME_PREFIX = "audio/";
const PDF_MIME = "application/pdf";

const TEXT_SIZE_LIMIT = 1024 * 1024; // 1MB

function extname(filename: string): string {
	const dot = filename.lastIndexOf(".");
	return dot === -1 ? "" : filename.slice(dot).toLowerCase();
}

export function getPreviewType(file: FileInfo): PreviewType | null {
	if (file.isDir) return null;

	const ext = extname(file.name);
	const mime = file.mimeType ?? "";

	// Markdown first (before generic text/* match)
	if (MARKDOWN_EXTENSIONS.includes(ext) || mime === "text/markdown") {
		return "markdown";
	}

	// Images
	if (mime.startsWith(IMAGE_MIME_PREFIX)) {
		return "image";
	}

	// Video
	if (mime.startsWith(VIDEO_MIME_PREFIX)) {
		return "video";
	}

	// Audio
	if (mime.startsWith(AUDIO_MIME_PREFIX)) {
		return "audio";
	}

	// PDF
	if (mime === PDF_MIME) {
		return "pdf";
	}

	// Text/code
	if (TEXT_MIME_PREFIXES.some((p) => mime.startsWith(p))) {
		return "text";
	}
	if (TEXT_MIME_EXACT.includes(mime)) {
		return "text";
	}

	return null;
}

export function canPreview(file: FileInfo): boolean {
	return getPreviewType(file) !== null;
}

export function isPreviewTooLarge(file: FileInfo): boolean {
	const type = getPreviewType(file);
	if (type === "text" || type === "markdown") {
		return file.size > TEXT_SIZE_LIMIT;
	}
	return false;
}

export function getPreviewUrl(path: string): string {
	return `/api/preview/${path}`;
}
