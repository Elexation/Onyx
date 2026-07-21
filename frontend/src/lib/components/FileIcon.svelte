<script lang="ts">
	import Folder from "@lucide/svelte/icons/folder";
	import File from "@lucide/svelte/icons/file";
	import FileText from "@lucide/svelte/icons/file-text";
	import Image from "@lucide/svelte/icons/image";
	import Video from "@lucide/svelte/icons/video";
	import Music from "@lucide/svelte/icons/music";
	import Archive from "@lucide/svelte/icons/archive";
	import FileCode from "@lucide/svelte/icons/file-code";

	let {
		mimeType = "",
		isDir = false,
		name = "",
		class: className = "size-4",
		strokeWidth = 2,
	}: {
		mimeType?: string;
		isDir?: boolean;
		name?: string;
		class?: string;
		strokeWidth?: number;
	} = $props();

	const CODE_TYPES = new Set([
		"application/json",
		"application/xml",
		"application/javascript",
		"application/typescript",
		"application/xhtml+xml",
	]);

	const DOC_TYPES = new Set([
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.ms-excel",
		"application/vnd.ms-powerpoint",
	]);

	const ARCHIVE_TYPES = new Set([
		"application/zip",
		"application/gzip",
		"application/x-tar",
		"application/x-7z-compressed",
		"application/x-rar-compressed",
		"application/x-bzip2",
	]);

	const EXT_COLOR: Record<string, string> = {
		pdf: "oklch(0.65 0.18 27)",
		md: "oklch(0.7 0.1 245)",
		json: "oklch(0.75 0.12 85)",
		js: "oklch(0.8 0.14 85)",
		mjs: "oklch(0.8 0.14 85)",
		cjs: "oklch(0.8 0.14 85)",
		ts: "oklch(0.7 0.12 245)",
		jsx: "oklch(0.7 0.12 245)",
		tsx: "oklch(0.7 0.12 245)",
		svelte: "oklch(0.7 0.18 35)",
		go: "oklch(0.7 0.13 220)",
		py: "oklch(0.75 0.12 85)",
		rs: "oklch(0.7 0.17 35)",
		html: "oklch(0.7 0.17 35)",
		htm: "oklch(0.7 0.17 35)",
		css: "oklch(0.7 0.12 245)",
		scss: "oklch(0.7 0.14 335)",
		yml: "oklch(0.7 0.12 335)",
		yaml: "oklch(0.7 0.12 335)",
		toml: "oklch(0.7 0.12 335)",
		xml: "oklch(0.7 0.12 335)",
		sh: "oklch(0.72 0.12 145)",
		bash: "oklch(0.72 0.12 145)",
		png: "oklch(0.78 0.14 145)",
		jpg: "oklch(0.78 0.14 145)",
		jpeg: "oklch(0.78 0.14 145)",
		webp: "oklch(0.78 0.14 145)",
		gif: "oklch(0.78 0.14 145)",
		avif: "oklch(0.78 0.14 145)",
		bmp: "oklch(0.78 0.14 145)",
		svg: "oklch(0.78 0.14 85)",
		mp4: "oklch(0.7 0.18 305)",
		mov: "oklch(0.7 0.18 305)",
		mkv: "oklch(0.7 0.18 305)",
		webm: "oklch(0.7 0.18 305)",
		avi: "oklch(0.7 0.18 305)",
		m4v: "oklch(0.7 0.18 305)",
		mp3: "oklch(0.75 0.14 320)",
		flac: "oklch(0.75 0.14 320)",
		wav: "oklch(0.75 0.14 320)",
		m4a: "oklch(0.75 0.14 320)",
		ogg: "oklch(0.75 0.14 320)",
		opus: "oklch(0.75 0.14 320)",
		aac: "oklch(0.75 0.14 320)",
		zip: "oklch(0.75 0.1 60)",
		tar: "oklch(0.75 0.1 60)",
		gz: "oklch(0.75 0.1 60)",
		"7z": "oklch(0.75 0.1 60)",
		rar: "oklch(0.75 0.1 60)",
		bz2: "oklch(0.75 0.1 60)",
	};

	const EXT_ICON: Record<string, typeof File> = {
		pdf: FileText, md: FileText, txt: FileText,
		doc: FileText, docx: FileText, xls: FileText, xlsx: FileText, ppt: FileText, pptx: FileText,
		png: Image, jpg: Image, jpeg: Image, webp: Image, gif: Image, avif: Image, bmp: Image, svg: Image,
		mp4: Video, mov: Video, mkv: Video, webm: Video, avi: Video, m4v: Video,
		mp3: Music, flac: Music, wav: Music, m4a: Music, ogg: Music, opus: Music, aac: Music,
		json: FileCode, js: FileCode, mjs: FileCode, cjs: FileCode,
		ts: FileCode, jsx: FileCode, tsx: FileCode, svelte: FileCode,
		go: FileCode, py: FileCode, rs: FileCode,
		html: FileCode, htm: FileCode, css: FileCode, scss: FileCode,
		yml: FileCode, yaml: FileCode, toml: FileCode, xml: FileCode,
		sh: FileCode, bash: FileCode,
		zip: Archive, tar: Archive, gz: Archive, "7z": Archive, rar: Archive, bz2: Archive,
	};

	function getIcon(mime: string, dir: boolean, n: string) {
		if (dir) return Folder;
		if (mime) {
			if (DOC_TYPES.has(mime)) return FileText;
			if (mime.startsWith("text/")) return FileText;
			if (mime.startsWith("image/")) return Image;
			if (mime.startsWith("video/")) return Video;
			if (mime.startsWith("audio/")) return Music;
			if (CODE_TYPES.has(mime)) return FileCode;
			if (ARCHIVE_TYPES.has(mime)) return Archive;
		}
		const ext = getExt(n);
		if (ext in EXT_ICON) return EXT_ICON[ext];
		return File;
	}

	function getExt(n: string): string {
		const i = n.lastIndexOf(".");
		return i > 0 ? n.slice(i + 1).toLowerCase() : "";
	}

	function getMimeColor(mime: string): string | undefined {
		if (mime.startsWith("image/")) return "oklch(0.78 0.14 145)";
		if (mime.startsWith("video/")) return "oklch(0.7 0.18 305)";
		if (mime.startsWith("audio/")) return "oklch(0.75 0.14 320)";
		if (mime.startsWith("text/")) return "oklch(0.7 0.1 245)";
		if (ARCHIVE_TYPES.has(mime)) return "oklch(0.75 0.1 60)";
		return undefined;
	}

	const Icon = $derived(getIcon(mimeType, isDir, name));
	const tint = $derived.by(() => {
		if (isDir) return undefined;
		const ext = getExt(name);
		if (EXT_COLOR[ext]) return EXT_COLOR[ext];
		return getMimeColor(mimeType);
	});
</script>

<Icon class={className} {strokeWidth} style={tint ? `color: ${tint}` : undefined} />
