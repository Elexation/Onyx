// hls.js wrapper used by VideoPreview for transcoded playback.
//
// Init order matters: attachMedia must fire first and loadSource is
// deferred until the MEDIA_ATTACHED event. Reversing the two is a known
// anti-pattern that causes inconsistent buffer behavior.
//
// The module is intended to be dynamically imported so hls.js stays out
// of the main bundle when the user never opens a transcoded video.

import Hls from "hls.js";

export type HlsLevel = {
	index: number;
	height: number;
	bitrate: number;
};

export type HlsHandle = {
	destroy: () => void;
	getLevels: () => HlsLevel[];
	getCurrentLevel: () => number;
	setLevel: (index: number) => void;
	setAutoLevelCap: (maxHeight: number) => void;
	onLevelsLoaded: (cb: (levels: HlsLevel[]) => void) => void;
	onLevelSwitched: (cb: (index: number) => void) => void;
	onFatalError: (cb: (data: unknown) => void) => void;
};

export function isHlsSupported(): boolean {
	return Hls.isSupported();
}

export function createHlsPlayer(videoEl: HTMLVideoElement, src: string): HlsHandle | null {
	if (!Hls.isSupported()) {
		// Safari plays HLS natively — caller can set videoEl.src directly.
		return null;
	}

	let fatalCb: ((data: unknown) => void) | null = null;
	const hls = new Hls();
	hls.attachMedia(videoEl);
	hls.on(Hls.Events.MEDIA_ATTACHED, () => {
		hls.loadSource(src);
	});
	hls.on(Hls.Events.ERROR, (_event, data) => {
		if (!data.fatal) return;
		console.error("[hls] fatal error", data);
		switch (data.type) {
			case Hls.ErrorTypes.NETWORK_ERROR:
				hls.startLoad();
				return;
			case Hls.ErrorTypes.MEDIA_ERROR:
				hls.recoverMediaError();
				return;
			default:
				hls.destroy();
				fatalCb?.(data);
		}
	});

	function snapshotLevels(): HlsLevel[] {
		return hls.levels.map((lvl, i) => ({
			index: i,
			height: lvl.height,
			bitrate: lvl.bitrate,
		}));
	}

	return {
		destroy: () => {
			hls.destroy();
		},
		getLevels: () => snapshotLevels(),
		getCurrentLevel: () => hls.currentLevel,
		setLevel: (index: number) => {
			// -1 = auto (ABR), >= 0 = locked manual level.
			if (index < 0) {
				hls.currentLevel = -1;
				return;
			}
			hls.currentLevel = index;
		},
		setAutoLevelCap: (maxHeight: number) => {
			// Clamp the ceiling used when currentLevel === -1 (auto mode).
			// maxHeight <= 0 disables the cap. hls.js expects a level index,
			// so translate from height → highest index whose level.height
			// is at or below the cap.
			if (maxHeight <= 0) {
				hls.autoLevelCapping = -1;
				return;
			}
			let cap = -1;
			for (let i = 0; i < hls.levels.length; i++) {
				if (hls.levels[i].height <= maxHeight) {
					if (cap < 0 || hls.levels[i].height > hls.levels[cap].height) {
						cap = i;
					}
				}
			}
			hls.autoLevelCapping = cap;
		},
		onLevelsLoaded: (cb) => {
			hls.on(Hls.Events.MANIFEST_PARSED, () => cb(snapshotLevels()));
		},
		onLevelSwitched: (cb) => {
			hls.on(Hls.Events.LEVEL_SWITCHED, (_e, data) => cb(data.level));
		},
		onFatalError: (cb) => {
			fatalCb = cb;
		},
	};
}

// canPlayHlsNatively is a Safari/iOS-only fallback — do NOT use it to
// gate MSE on Chrome. Chrome reports "maybe" for this MIME type but its
// demuxer cannot actually parse m3u8, so any caller must check
// Hls.isSupported() first and fall back here only when MSE is
// unavailable.
export function canPlayHlsNatively(videoEl: HTMLVideoElement): boolean {
	return videoEl.canPlayType("application/vnd.apple.mpegurl") !== "";
}
