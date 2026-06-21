import { encodeFilePath } from "$lib/utils";

export type ProbeInfo = {
	codec: string;
	width: number;
	height: number;
	duration: number;
	bitrate: number;
	framerate: number;
	needsTranscode: boolean;
};

export type ProbeStatus = "ok" | "no-video" | "error";

export type ProbeResult = { status: ProbeStatus; info?: ProbeInfo };

export function getStreamInfoUrl(path: string, infoBase = "/api/stream/info"): string {
	return `${infoBase}${encodeFilePath(path)}`;
}

export async function fetchProbeInfo(path: string, infoBase = "/api/stream/info"): Promise<ProbeResult> {
	try {
		const res = await fetch(getStreamInfoUrl(path, infoBase), { credentials: "same-origin" });
		if (res.status === 415) return { status: "no-video" };
		if (!res.ok) return { status: "error" };
		const info = (await res.json()) as ProbeInfo;
		return { status: "ok", info };
	} catch {
		return { status: "error" };
	}
}

// contentTypeFor builds a MIME+codecs string for navigator.mediaCapabilities.
// Returns null if the codec isn't one we know how to format.
function contentTypeFor(codec: string): string | null {
	switch (codec) {
		case "h264":
			return 'video/mp4; codecs="avc1.640028"';
		case "hevc":
		case "h265":
			return 'video/mp4; codecs="hev1.1.6.L120.90"';
		case "vp8":
			return 'video/webm; codecs="vp8"';
		case "vp9":
			return 'video/webm; codecs="vp09.00.10.08"';
		case "av1":
			return 'video/mp4; codecs="av01.0.05M.08"';
	}
	return null;
}

// Fallback decision when navigator.mediaCapabilities is unavailable.
// Covers the historically-safe native-playback set.
const FALLBACK_NATIVE_CODECS = new Set(["h264", "vp8", "vp9"]);

export async function canPlayNative(info: ProbeInfo): Promise<boolean> {
	const mc = (navigator as Navigator & { mediaCapabilities?: MediaCapabilities }).mediaCapabilities;
	if (!mc || typeof mc.decodingInfo !== "function") {
		return FALLBACK_NATIVE_CODECS.has(info.codec);
	}

	const contentType = contentTypeFor(info.codec);
	if (!contentType) return false;

	try {
		const result = await mc.decodingInfo({
			type: "file",
			video: {
				contentType,
				width: info.width || 1920,
				height: info.height || 1080,
				bitrate: info.bitrate || 5_000_000,
				framerate: info.framerate || 30,
			},
		});
		return result.supported && result.smooth;
	} catch {
		return FALLBACK_NATIVE_CODECS.has(info.codec);
	}
}
