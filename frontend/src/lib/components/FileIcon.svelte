<script lang="ts">
	import {
		Folder,
		File,
		FileText,
		Image,
		Video,
		Music,
		Archive,
		FileCode,
	} from "lucide-svelte";

	let { mimeType = "", isDir = false, class: className = "size-4" }: {
		mimeType?: string;
		isDir?: boolean;
		class?: string;
	} = $props();

	const CODE_TYPES = ["application/json", "application/xml", "application/javascript", "application/typescript", "application/xhtml+xml"];

	const ARCHIVE_TYPES = ["application/zip", "application/gzip", "application/x-tar", "application/x-7z-compressed", "application/x-rar-compressed", "application/x-bzip2"];

	function getIcon(mime: string, dir: boolean) {
		if (dir) return Folder;
		if (!mime) return File;
		if (mime.startsWith("text/")) return FileText;
		if (mime.startsWith("image/")) return Image;
		if (mime.startsWith("video/")) return Video;
		if (mime.startsWith("audio/")) return Music;
		if (CODE_TYPES.includes(mime)) return FileCode;
		if (ARCHIVE_TYPES.includes(mime)) return Archive;
		return File;
	}

	const Icon = $derived(getIcon(mimeType, isDir));
</script>

<Icon class={className} />
