<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { encodeFilePath } from "$lib/utils";
	import { formatMediaTime } from "$lib/utils/format.js";
	import { fetchProbeInfo, canPlayNative, type ProbeInfo } from "$lib/media/capabilities";
	import { getSettings } from "$lib/api/settings";
	import type { FileInfo } from "$lib/types";
	import type { HlsHandle, HlsLevel } from "$lib/media/hls";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import PlayIcon from "@lucide/svelte/icons/play";
	import PauseIcon from "@lucide/svelte/icons/pause";
	import Volume2Icon from "@lucide/svelte/icons/volume-2";
	import VolumeXIcon from "@lucide/svelte/icons/volume-x";
	import MaximizeIcon from "@lucide/svelte/icons/maximize";
	import MinimizeIcon from "@lucide/svelte/icons/minimize";
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import ChevronsLeftIcon from "@lucide/svelte/icons/chevrons-left";
	import ChevronsRightIcon from "@lucide/svelte/icons/chevrons-right";
	import DownloadIcon from "@lucide/svelte/icons/download";
	import XIcon from "@lucide/svelte/icons/x";
	import TriangleAlertIcon from "@lucide/svelte/icons/triangle-alert";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import { fade } from "svelte/transition";
	import { Button } from "$lib/components/ui/button/index.js";
	import "./media-controls.css";

	let { file, onclose, ondownload, url, streamBase, portalTarget }: { file: FileInfo; onclose: () => void; ondownload?: () => void; url?: string; streamBase?: string; portalTarget?: HTMLElement | null } = $props();

	type PlaybackMode = "loading" | "native" | "transcode-required" | "no-video";

	let videoEl = $state<HTMLVideoElement | null>(null);
	let containerEl = $state<HTMLDivElement | null>(null);
	let isFullscreen = $state(false);
	let playing = $state(false);
	let currentTime = $state(0);
	let duration = $state(0);
	let volume = $state(1);
	let muted = $state(false);
	let bufferedEnd = $state(0);
	let showControls = $state(true);
	let failed = $state(false);
	let probeInfo = $state<ProbeInfo | null>(null);
	let controlsTimer: ReturnType<typeof setTimeout> | null = null;
	let lastSaveTime = 0;

	// Scrub state — separate from playhead so drag motion doesn't hit
	// videoEl.currentTime on every input event. Commit happens on `change`
	// (pointerup / Enter / blur), one write per gesture.
	let scrubbing = $state(false);
	let scrubTime = $state(0);
	let seekBarHovered = $state(false);
	let hoverPercent = $state(0);
	let playPauseAction = $state<'play' | 'pause' | null>(null);
	let playPauseKey = $state(0);
	let playPauseTimer: ReturnType<typeof setTimeout> | null = null;
	let buffering = $state(false);
	let volumeFlash = $state(false);
	let volumeFlashTimer: ReturnType<typeof setTimeout> | null = null;

	// Arrow-key seek accumulates into a settle timer so held/mashed
	// presses produce one currentTime write, not one per keydown. Held
	// keys throttle accumulation to ~6.7/sec so the offset grows at a
	// usable rate instead of tracking the OS key-repeat frequency.
	// Commit fires only when all arrows are released — committing mid-hold
	// causes a visible jolt as the UI snaps between pre- and post-commit.
	let keySeekOffset = $state(0);
	let keySeekTimer: ReturnType<typeof setTimeout> | null = null;
	const heldArrows = new Set<string>();
	let lastKeyAccumAt = 0;
	const KEY_SEEK_SETTLE_MS = 400;
	const KEY_SEEK_ACCUM_MS = 80;

	let detectedMode = $state<PlaybackMode>("loading");
	let nativeSupported = $state(false);
	let userMode = $state<"original" | "transcode" | null>(null);
	let userPickedHeight = $state<number | null>(null);
	let pendingSeek = $state<number | null>(null);
	let pendingPaused = $state(false);

	let hlsHandle: HlsHandle | null = null;
	let qualityLevels = $state<HlsLevel[]>([]);
	let selectedQuality = $state<number>(-1);
	let currentAutoLevel = $state<number>(-1);
	let defaultQualityCeiling = $state<number>(1080);

	const STORAGE_PREFIX = "onyx-video-pos:";
	const VOLUME_KEY = "onyx-video-volume";
	const QUALITY_LADDER = [2160, 1440, 1080, 720, 480];

	const playback = $derived.by(() => {
		if (detectedMode === "loading" || detectedMode === "no-video") return detectedMode;
		if (userMode === "transcode") return "transcode-required";
		if (userMode === "original" && nativeSupported) return "native";
		return detectedMode;
	});

	const lowerQualities = $derived(
		probeInfo?.height
			? QUALITY_LADDER.filter((h) => h < probeInfo!.height)
			: []
	);

	const availableQualities = $derived(
		probeInfo?.height
			? qualityLevels.filter((l) => l.height <= probeInfo!.height)
			: qualityLevels
	);

	const showQualityMenu = $derived(
		nativeSupported
			? lowerQualities.length > 0
			: availableQualities.length > 0
	);

	const qualityButtonLabel = $derived.by(() => {
		if (nativeSupported) {
			if (userMode === "transcode" && userPickedHeight) {
				return `${userPickedHeight}p`;
			}
			return probeInfo?.height ? `${probeInfo.height}p` : "";
		}
		if (availableQualities.length === 0) return "";
		if (selectedQuality < 0) {
			if (currentAutoLevel >= 0 && currentAutoLevel < qualityLevels.length) {
				return `Auto (${qualityLabel(qualityLevels[currentAutoLevel])})`;
			}
			return "Auto";
		}
		if (selectedQuality < qualityLevels.length) {
			return qualityLabel(qualityLevels[selectedQuality]);
		}
		return "";
	});

	const displayTime = $derived.by(() => {
		if (scrubbing) return scrubTime;
		if (keySeekOffset !== 0) {
			return Math.max(0, Math.min(duration, currentTime + keySeekOffset));
		}
		return currentTime;
	});
	const seekPercent = $derived(duration > 0 ? (displayTime / duration) * 100 : 0);
	const bufferedPercent = $derived(duration > 0 ? (bufferedEnd / duration) * 100 : 0);
	const hoverTime = $derived(duration > 0 ? (hoverPercent / 100) * duration : 0);

	// Force the bottom bar visible while any scrub is in flight; the
	// normal 3s auto-hide resumes once the gesture settles.
	let controlsFocused = $state(false);
	let qualityMenuOpen = $state(false);
	let cogPressed = $state(false);
	let cogHovered = $state(false);

	const cogStyle = $derived.by(() => {
		if (cogPressed) return 'background-color: rgb(255 255 255 / 0.2); color: white';
		if (cogHovered) return 'background-color: rgb(255 255 255 / 0.1); color: white';
		return undefined;
	});

	// Only clears cursor writes this component made, so an unrelated one survives.
	let cogCursorSet = false;
	function setCogCursor(on: boolean) {
		if (on === cogCursorSet) return;
		cogCursorSet = on;
		document.documentElement.style.cursor = on ? 'pointer' : '';
	}

	function pressTrack(node: HTMLElement) {
		// bits-ui drops page pointer-events while the menu is open, so hover and
		// press state can only come from hit-testing document-level moves. The rect
		// is cached because that hit-test runs on every move across the whole page.
		let rect: DOMRect | null = null;
		const invalidate = () => { rect = null; };

		const isOverBtn = (x: number, y: number) => {
			if (!rect) rect = node.querySelector('button')?.getBoundingClientRect() ?? null;
			if (!rect) return false;
			return x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
		};

		const onDown = (e: PointerEvent) => {
			if (!isOverBtn(e.clientX, e.clientY)) return;
			cogPressed = true;
		};
		const onUp = (e: PointerEvent) => {
			if (cogPressed && qualityMenuOpen && isOverBtn(e.clientX, e.clientY)) {
				qualityMenuOpen = false;
			}
			cogPressed = false;
		};
		const onMove = (e: PointerEvent) => {
			const over = isOverBtn(e.clientX, e.clientY);
			cogHovered = over;
			setCogCursor(over && qualityMenuOpen);
		};

		document.addEventListener('pointerdown', onDown, true);
		document.addEventListener('pointerup', onUp, true);
		document.addEventListener('pointermove', onMove);
		window.addEventListener('resize', invalidate);
		document.addEventListener('fullscreenchange', invalidate);

		return { destroy() {
			document.removeEventListener('pointerdown', onDown, true);
			document.removeEventListener('pointerup', onUp, true);
			document.removeEventListener('pointermove', onMove);
			window.removeEventListener('resize', invalidate);
			document.removeEventListener('fullscreenchange', invalidate);
			setCogCursor(false);
		}};
	}
	const controlsVisible = $derived(showControls || scrubbing || keySeekOffset !== 0 || controlsFocused || qualityMenuOpen);

	// !failed keeps the pointer over the retry overlay, which outlives the auto-hide timer
	const cursorHidden = $derived(!controlsVisible && !failed);

	// Fullscreen promotes containerEl above the dialog in the top layer, so menu
	// content portaled to the dialog paints behind the video and is unreachable.
	const menuPortalTarget = $derived(isFullscreen ? containerEl : portalTarget);

	function restorePosition() {
		if (!videoEl) return;
		try {
			const raw = localStorage.getItem(STORAGE_PREFIX + file.path);
			if (!raw) return;
			const saved = JSON.parse(raw);
			if (saved.modTime === file.modTime && saved.time > 0) {
				videoEl.currentTime = saved.time;
			} else {
				localStorage.removeItem(STORAGE_PREFIX + file.path);
			}
		} catch { /* ignore parse errors */ }
	}

	function savePosition() {
		if (!videoEl || videoEl.currentTime < 1) return;
		const now = Date.now();
		if (now - lastSaveTime < 5000) return;
		lastSaveTime = now;
		try {
			localStorage.setItem(
				STORAGE_PREFIX + file.path,
				JSON.stringify({ time: videoEl.currentTime, modTime: file.modTime }),
			);
		} catch { /* storage full */ }
	}

	function clearPosition() {
		localStorage.removeItem(STORAGE_PREFIX + file.path);
	}

	function restoreVolume() {
		if (!videoEl) return;
		try {
			const raw = localStorage.getItem(VOLUME_KEY);
			if (!raw) return;
			const saved = JSON.parse(raw);
			videoEl.volume = saved.volume ?? 1;
			videoEl.muted = saved.muted ?? false;
			volume = videoEl.volume;
			muted = videoEl.muted;
		} catch { /* ignore */ }
	}

	function saveVolume() {
		try {
			localStorage.setItem(VOLUME_KEY, JSON.stringify({ volume, muted }));
		} catch { /* storage full */ }
	}

	function flashPlayPause(action: 'play' | 'pause') {
		playPauseAction = action;
		playPauseKey++;
		if (playPauseTimer) clearTimeout(playPauseTimer);
		playPauseTimer = setTimeout(() => { playPauseAction = null; }, 600);
	}

	function flashVolume() {
		volumeFlash = true;
		if (volumeFlashTimer) clearTimeout(volumeFlashTimer);
		volumeFlashTimer = setTimeout(() => { volumeFlash = false; }, 1000);
	}

	function togglePlay(flash = true) {
		if (!videoEl || failed) return;
		const willPlay = videoEl.paused;
		if (willPlay) videoEl.play().catch(() => { failed = true; });
		else videoEl.pause();
		if (flash) flashPlayPause(willPlay ? 'play' : 'pause');
	}

	function toggleMute() {
		if (!videoEl) return;
		if (videoEl.muted || videoEl.volume === 0) {
			videoEl.muted = false;
			if (videoEl.volume === 0) videoEl.volume = 0.25;
		} else {
			videoEl.muted = true;
		}
		// volumechange is async; the flash reads these on the same tick
		muted = videoEl.muted;
		volume = videoEl.volume;
	}

	// Toggle on the first click and let dblclick undo it, so a plain click responds
	// immediately instead of waiting out a dblclick discrimination timer.
	function handleVideoClick(e: MouseEvent) {
		if (e.detail > 1) return;
		togglePlay();
	}

	function handleVideoDblClick() {
		togglePlay(false);
		if (playPauseTimer) clearTimeout(playPauseTimer);
		playPauseAction = null;
		toggleFullscreen();
	}

	function toggleFullscreen() {
		if (document.fullscreenElement) {
			document.exitFullscreen();
			return;
		}
		if (containerEl && document.fullscreenEnabled) {
			containerEl.requestFullscreen();
			return;
		}
		if (videoEl && "webkitEnterFullscreen" in videoEl) {
			(videoEl as any).webkitEnterFullscreen();
		}
	}

	function queueKeySeek(delta: number, force: boolean) {
		if (!videoEl) return;
		const now = Date.now();
		if (!force && now - lastKeyAccumAt < KEY_SEEK_ACCUM_MS) return;
		keySeekOffset += delta;
		lastKeyAccumAt = now;
		// Cancel any pending commit — we'll restart the settle timer on keyup.
		if (keySeekTimer) {
			clearTimeout(keySeekTimer);
			keySeekTimer = null;
		}
	}

	function commitKeySeek() {
		if (videoEl && keySeekOffset !== 0) {
			const target = Math.max(0, Math.min(duration, videoEl.currentTime + keySeekOffset));
			videoEl.currentTime = target;
			// Sync local state so displayTime doesn't briefly snap back to
			// the pre-commit value before ontimeupdate catches up.
			currentTime = target;
		}
		keySeekOffset = 0;
		keySeekTimer = null;
		resetControlsTimer();
	}

	function resetControlsTimer() {
		showControls = true;
		if (controlsTimer) clearTimeout(controlsTimer);
		if (playing) {
			controlsTimer = setTimeout(() => { showControls = false; }, 3000);
		}
	}

	function handleTimeUpdate() {
		if (!videoEl) return;
		currentTime = videoEl.currentTime;
		if (videoEl.buffered.length > 0) {
			bufferedEnd = videoEl.buffered.end(videoEl.buffered.length - 1);
		}
		savePosition();
	}

	function handleSeekInput(e: Event) {
		scrubbing = true;
		scrubTime = Number((e.target as HTMLInputElement).value);
	}

	function handleSeekChange(e: Event) {
		if (!videoEl) return;
		const target = Number((e.target as HTMLInputElement).value);
		videoEl.currentTime = target;
		currentTime = target;
		scrubTime = target;
		scrubbing = false;
		resetControlsTimer();
	}

	function touchSeek(node: HTMLElement) {
		let barRect: DOMRect | null = null;
		let startTouchX = 0;
		let startTime = 0;

		function onPointerDown(e: PointerEvent) {
			if (e.pointerType === 'touch') e.preventDefault();
		}

		function onStart(e: TouchEvent) {
			if (!duration) return;
			e.preventDefault();
			barRect = node.getBoundingClientRect();
			startTouchX = e.touches[0].clientX;
			startTime = currentTime;
			scrubbing = true;
			scrubTime = currentTime;
			document.addEventListener('touchmove', onMove, { passive: false });
			document.addEventListener('touchend', onEnd);
		}

		function onMove(e: TouchEvent) {
			if (!scrubbing || !duration || !barRect) return;
			e.preventDefault();
			const deltaX = e.touches[0].clientX - startTouchX;
			scrubTime = Math.max(0, Math.min(duration, startTime + (deltaX / barRect.width) * duration));
		}

		function onEnd() {
			if (videoEl && scrubbing) {
				videoEl.currentTime = scrubTime;
				currentTime = scrubTime;
			}
			scrubbing = false;
			barRect = null;
			resetControlsTimer();
			document.removeEventListener('touchmove', onMove);
			document.removeEventListener('touchend', onEnd);
		}

		node.addEventListener('pointerdown', onPointerDown);
		node.addEventListener('touchstart', onStart, { passive: false });

		return {
			destroy() {
				node.removeEventListener('pointerdown', onPointerDown);
				node.removeEventListener('touchstart', onStart);
				document.removeEventListener('touchmove', onMove);
				document.removeEventListener('touchend', onEnd);
			}
		};
	}

	function handleSeekBarMove(e: PointerEvent) {
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		if (e.clientY < rect.top - 16 || e.clientY > rect.bottom + 16) {
			seekBarHovered = false;
			return;
		}
		hoverPercent = Math.max(0, Math.min(100, ((e.clientX - rect.left) / rect.width) * 100));
		seekBarHovered = true;
	}

	function handleVolumeInput(e: Event) {
		if (!videoEl) return;
		const v = Number((e.target as HTMLInputElement).value);
		videoEl.volume = v;
		volume = v;
		if (v > 0 && muted) {
			videoEl.muted = false;
		}
	}

	function handleKeyup(e: KeyboardEvent) {
		if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
		heldArrows.delete(e.key);
		if (heldArrows.size > 0) return;
		if (keySeekTimer) clearTimeout(keySeekTimer);
		keySeekTimer = setTimeout(commitKeySeek, KEY_SEEK_SETTLE_MS);
	}

	function handleWindowBlur() {
		// Alt-tab / focus loss while arrows held — we'll never get keyup.
		// Commit whatever accumulated and clear held state.
		if (heldArrows.size === 0 && keySeekOffset === 0) return;
		heldArrows.clear();
		if (keySeekTimer) clearTimeout(keySeekTimer);
		commitKeySeek();
	}

	function handleKeydown(e: KeyboardEvent) {
		// The open menu owns the keyboard, but selecting an item closes it before
		// this bubbles to window, so Space and Enter need the origin check too.
		if (qualityMenuOpen) return;
		if ((e.target as HTMLElement)?.closest?.('[data-slot="dropdown-menu-content"]')) return;
		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
		if (tag === "BUTTON" && (e.key === " " || e.key === "Enter")) return;

		switch (e.key) {
			case " ":
				e.preventDefault();
				togglePlay();
				break;
			case "ArrowLeft":
				e.preventDefault();
				heldArrows.add(e.key);
				queueKeySeek(-10, !e.repeat);
				break;
			case "ArrowRight":
				e.preventDefault();
				heldArrows.add(e.key);
				queueKeySeek(10, !e.repeat);
				break;
			case "ArrowUp":
				e.preventDefault();
				if (videoEl) {
					videoEl.volume = Math.min(1, volume + 0.1);
					volume = videoEl.volume;
					flashVolume();
				}
				break;
			case "ArrowDown":
				e.preventDefault();
				if (videoEl) {
					videoEl.volume = Math.max(0, volume - 0.1);
					volume = videoEl.volume;
					flashVolume();
				}
				break;
			case "f":
			case "F":
				e.preventDefault();
				toggleFullscreen();
				break;
			case "m":
			case "M":
				e.preventDefault();
				toggleMute();
				flashVolume();
				break;
		}
	}

	function pickQuality(index: number) {
		if (!hlsHandle) return;
		selectedQuality = index;
		hlsHandle.setLevel(index);
	}

	function qualityLabel(level: HlsLevel): string {
		return `${level.height}p`;
	}

	function switchToOriginal() {
		if (!videoEl) return;
		if (pendingSeek === null) {
			pendingSeek = videoEl.currentTime;
			pendingPaused = videoEl.paused;
		}
		videoEl.pause();
		userMode = "original";
		userPickedHeight = null;
		selectedQuality = -1;
		failed = false;
	}

	function switchToTranscode(height: number) {
		if (!videoEl) return;
		userPickedHeight = height;

		if (userMode === "transcode" && hlsHandle) {
			const level = qualityLevels.find((l) => l.height === height);
			if (level) {
				selectedQuality = level.index;
				hlsHandle.setLevel(level.index);
			}
			return;
		}

		if (pendingSeek === null) {
			pendingSeek = videoEl.currentTime;
			pendingPaused = videoEl.paused;
		}
		videoEl.pause();
		userMode = "transcode";
		failed = false;
	}

	// --- Effects ---

	// Closing by Escape or outside-click leaves no pointermove to clear the cursor.
	$effect(() => {
		if (!qualityMenuOpen) setCogCursor(false);
	});

	$effect(() => {
		const el = videoEl;
		if (!el) return;
		restoreVolume();
		return () => {
			el.pause();
			if (controlsTimer) clearTimeout(controlsTimer);
			if (keySeekTimer) clearTimeout(keySeekTimer);
			if (playPauseTimer) clearTimeout(playPauseTimer);
			if (volumeFlashTimer) clearTimeout(volumeFlashTimer);
		};
	});

	$effect(() => {
		let cancelled = false;
		getSettings()
			.then((s) => {
				if (cancelled) return;
				const raw = s.values["playback.default_quality_ceiling"];
				const n = raw ? parseInt(raw, 10) : NaN;
				if (!isNaN(n)) {
					defaultQualityCeiling = n;
					hlsHandle?.setAutoLevelCap(n);
				}
			})
			.catch(() => {});
		return () => { cancelled = true; };
	});

	$effect(() => {
		let cancelled = false;
		const filePath = file.path;
		if (url && !streamBase) {
			detectedMode = "native";
			nativeSupported = true;
			return;
		}
		detectedMode = "loading";
		nativeSupported = false;
		userMode = null;
		userPickedHeight = null;
		pendingSeek = null;
		probeInfo = null;
		const infoBase = streamBase ? `${streamBase}/info` : "/api/stream/info";
		fetchProbeInfo(filePath, infoBase).then(async (result) => {
			if (cancelled) return;
			if (result.status === "no-video") {
				detectedMode = "no-video";
				return;
			}
			if (result.status !== "ok" || !result.info) {
				detectedMode = "native";
				nativeSupported = true;
				return;
			}
			probeInfo = result.info;
			const native = await canPlayNative(result.info);
			if (cancelled) return;
			nativeSupported = native;
			detectedMode = native ? "native" : "transcode-required";
		});
		return () => { cancelled = true; };
	});

	$effect(() => {
		const el = videoEl;
		if (!el) return;
		const mode = playback;
		const filePath = file.path;

		if (mode === "native") {
			el.src = url ?? getPreviewUrl(filePath);
			return;
		}

		if (mode !== "transcode-required") return;

		const masterBase = streamBase ? `${streamBase}/master` : "/api/stream/master";
		const masterUrl = `${masterBase}${encodeFilePath(filePath)}`;
		let localHandle: HlsHandle | null = null;
		let cancelled = false;

		(async () => {
			const { createHlsPlayer, isHlsSupported, canPlayHlsNatively } = await import("$lib/media/hls");
			if (cancelled) return;
			if (isHlsSupported()) {
				const handle = createHlsPlayer(el, masterUrl);
				if (cancelled) {
					handle?.destroy();
					return;
				}
				if (!handle) {
					failed = true;
					return;
				}
				localHandle = handle;
				hlsHandle = handle;
				handle.onLevelsLoaded((levels) => {
					qualityLevels = levels;
					handle.setAutoLevelCap(defaultQualityCeiling);
					if (userPickedHeight !== null) {
						const level = levels.find((l) => l.height === userPickedHeight);
						if (level) {
							selectedQuality = level.index;
							handle.setLevel(level.index);
						}
					}
				});
				handle.onLevelSwitched((idx) => {
					currentAutoLevel = idx;
				});
				handle.onFatalError(() => {
					failed = true;
				});
				return;
			}
			if (canPlayHlsNatively(el)) {
				el.src = masterUrl;
				return;
			}
			failed = true;
		})();

		return () => {
			cancelled = true;
			localHandle?.destroy();
			if (hlsHandle === localHandle) {
				hlsHandle = null;
				qualityLevels = [];
				selectedQuality = -1;
				currentAutoLevel = -1;
			}
		};
	});
</script>

<svelte:window onkeydown={handleKeydown} onkeyup={handleKeyup} onblur={handleWindowBlur} />
<svelte:document onfullscreenchange={() => { isFullscreen = document.fullscreenElement === containerEl; }} />

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	bind:this={containerEl}
	class="group relative flex flex-1 items-center justify-center overflow-hidden bg-black"
	class:cursor-none={cursorHidden}
	onmousemove={resetControlsTimer}
	onmouseleave={() => { if (playing) showControls = false; }}
>
	{#if playback === "loading"}
		<p class="text-[15px] text-muted-foreground" data-preview-content>Loading…</p>
	{:else if playback === "no-video"}
		<p class="text-[15px] text-muted-foreground" data-preview-content>No playable video stream in this file.</p>
	{:else}
	<!-- svelte-ignore a11y_media_has_caption -->
	<video
		bind:this={videoEl}
		class="h-full w-full object-contain"
		playsinline
		preload="metadata"
		data-preview-content
		onclick={handleVideoClick}
		ondblclick={handleVideoDblClick}
		onplay={() => { playing = true; if (videoEl && videoEl.readyState < 3) buffering = true; resetControlsTimer(); }}
		onpause={() => { playing = false; buffering = false; showControls = true; if (controlsTimer) clearTimeout(controlsTimer); }}
		onwaiting={() => { buffering = true; }}
		onplaying={() => { buffering = false; }}
		oncanplay={() => { buffering = false; }}
		ontimeupdate={handleTimeUpdate}
		onprogress={() => { if (videoEl && videoEl.buffered.length > 0) bufferedEnd = videoEl.buffered.end(videoEl.buffered.length - 1); }}
		onloadedmetadata={() => {
			if (videoEl) {
				duration = videoEl.duration;
				if (pendingSeek !== null) {
					if (pendingSeek > 0) videoEl.currentTime = pendingSeek;
					pendingSeek = null;
					if (!pendingPaused) {
						videoEl.play().catch(() => { failed = true; });
					}
					pendingPaused = false;
				} else {
					restorePosition();
				}
			}
		}}
		onvolumechange={() => { if (videoEl) { volume = videoEl.volume; muted = videoEl.muted; saveVolume(); } }}
		onended={() => { playing = false; showControls = true; if (controlsTimer) clearTimeout(controlsTimer); clearPosition(); }}
		onerror={() => { if (!hlsHandle && pendingSeek === null) failed = true; }}
	></video>

	{#if failed}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="absolute inset-0 flex items-center justify-center bg-black/80" onclick={onclose}>
			<div class="flex flex-col items-center gap-3" onclick={(e) => e.stopPropagation()}>
				<TriangleAlertIcon class="size-8 text-muted-foreground" />
				<p class="text-base text-white">Video playback failed</p>
				<Button
					variant="ghost"
					size="sm"
					class="mt-1 gap-2 px-4 text-meta text-white/80 hover:bg-white/10 hover:text-white"
					onclick={() => { failed = false; }}
				>
					<RotateCcwIcon class="size-3.5" />
					Try again
				</Button>
			</div>
		</div>
	{:else}
{#if keySeekOffset !== 0}
			<div
				class="pointer-events-none absolute left-1/2 top-8 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-black/70 px-4 py-2 text-meta tabular-nums text-white backdrop-blur-sm"
				transition:fade={{ duration: 120 }}
			>
				{#if keySeekOffset < 0}
					<ChevronsLeftIcon class="size-4" />
					<span class="tabular-nums">{Math.abs(keySeekOffset)}s</span>
				{:else}
					<span class="tabular-nums">{keySeekOffset}s</span>
					<ChevronsRightIcon class="size-4" />
				{/if}
			</div>
		{:else if volumeFlash}
			<div
				class="pointer-events-none absolute left-1/2 top-8 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-black/70 px-4 py-2 text-meta text-white backdrop-blur-sm"
				transition:fade={{ duration: 120 }}
			>
				{#if muted || volume === 0}
					<VolumeXIcon class="size-4" />
					<span>Muted</span>
				{:else}
					<Volume2Icon class="size-4" />
					<span class="tabular-nums">{Math.round(volume * 100)}%</span>
				{/if}
			</div>
		{/if}

		{#if buffering}
			<div class="pointer-events-none absolute inset-0 flex items-center justify-center">
				<div class="buffering-spinner size-10 rounded-full border-[3px] border-white/30 border-t-white"></div>
			</div>
		{:else if playPauseAction}
			{#key playPauseKey}
				<div class="pointer-events-none absolute inset-0 flex items-center justify-center">
					<div class="play-pause-flash flex size-20 items-center justify-center rounded-full bg-black/60 text-white">
						{#if playPauseAction === 'play'}
							<PlayIcon class="size-10 translate-x-0.5" />
						{:else}
							<PauseIcon class="size-10" />
						{/if}
					</div>
				</div>
			{/key}
		{/if}
	{/if}

	{#if !failed}
	<div
		class="absolute bottom-0 left-0 right-0 flex flex-col gap-2 bg-black/70 px-3 py-2 backdrop-blur-sm transition-opacity duration-200"
		class:opacity-0={!controlsVisible}
		class:pointer-events-none={!controlsVisible}
		onclick={(e) => e.stopPropagation()}
		onfocusin={(e) => {
			// Click-focus stays on the button so focusout never fires; only
			// keyboard focus may pin the bar open.
			controlsFocused = (e.target as HTMLElement).matches(":focus-visible");
			if (!controlsFocused) resetControlsTimer();
		}}
		onfocusout={(e) => {
			if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
				controlsFocused = false;
				resetControlsTimer();
			}
		}}
	>
		<div
			class="seek-bar relative w-full cursor-pointer"
			onpointermove={handleSeekBarMove}
			onpointerleave={() => { seekBarHovered = false; }}
		>
			<div class="relative overflow-hidden rounded-full bg-white/20 transition-all duration-150 {seekBarHovered || scrubbing ? 'h-2.5' : 'h-1.5'}">
				{#if seekBarHovered && !scrubbing}
					<div
						class="absolute left-0 top-0 h-full bg-white/50"
						style="width: {Math.min(hoverPercent, bufferedPercent)}%"
					></div>
					{#if hoverPercent < bufferedPercent}
						<div
							class="absolute top-0 h-full bg-white/30"
							style="left: {hoverPercent}%; width: {bufferedPercent - hoverPercent}%"
						></div>
					{/if}
					{#if hoverPercent > bufferedPercent}
						<div
							class="absolute top-0 h-full bg-white/15"
							style="left: {bufferedPercent}%; width: {hoverPercent - bufferedPercent}%"
						></div>
					{/if}
				{:else}
					<div
						class="absolute left-0 top-0 h-full bg-white/30"
						style="width: {bufferedPercent}%"
					></div>
				{/if}
				<div
					class="absolute left-0 top-0 h-full bg-accent-brand"
					style="width: {seekPercent}%"
				></div>
			</div>
			<div
				class="absolute top-1/2 z-[1] -translate-x-1/2 -translate-y-1/2 rounded-full bg-accent-brand transition-[width,height] duration-150"
				style="left: {seekPercent}%; width: {seekBarHovered || scrubbing ? 20 : 16}px; height: {seekBarHovered || scrubbing ? 20 : 16}px"
			></div>
			{#if seekBarHovered && !scrubbing}
				<div
					class="pointer-events-none absolute z-[3] -translate-x-1/2 rounded bg-black/80 px-2 py-1 text-xs tabular-nums text-white"
					style="left: {hoverPercent}%; bottom: calc(100% + 8px)"
				>
					{formatMediaTime(hoverTime)}
				</div>
			{/if}
			<input
				use:touchSeek
				type="range"
				min="0"
				max={duration}
				step="0.1"
				value={displayTime}
				oninput={handleSeekInput}
				onchange={handleSeekChange}
				class="absolute inset-x-0 -top-[2.25rem] h-[calc(100%+2.25rem)] z-[2] w-full cursor-pointer opacity-0"
			/>
		</div>

		<div class="flex items-center gap-2">
			<Button variant="ghost" size="icon-touch" class="text-white/80 hover:bg-transparent hover:text-white"
				onclick={() => togglePlay()}
				aria-label={playing ? "Pause" : "Play"}
			>
				{#if playing}
					<PauseIcon class="size-5" />
				{:else}
					<PlayIcon class="size-5" />
				{/if}
			</Button>

			<span class="shrink-0 text-meta tabular-nums text-white/80">
				{formatMediaTime(displayTime)} / {formatMediaTime(duration)}
			</span>

			<div class="flex-1"></div>

			{#if showQualityMenu}
				<DropdownMenu.Root bind:open={qualityMenuOpen}>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<div use:pressTrack class="contents">
								<Button
									{...props}
									onpointerdown={() => {}}
									onclick={() => { qualityMenuOpen = !qualityMenuOpen; }}
									variant="ghost"
									size="icon-touch"
									aria-label="Quality settings"
									class="cog-button text-white/80 hover:bg-white/10 hover:text-white aria-expanded:bg-transparent aria-expanded:hover:bg-white/10"
									style={cogStyle}
								>
									<SettingsIcon class="size-5 transition-transform duration-200 {qualityMenuOpen ? 'rotate-[25deg]' : ''}" />
								</Button>
							</div>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="min-w-36" portalProps={menuPortalTarget ? { to: menuPortalTarget } : undefined}>
						{#if nativeSupported}
							<DropdownMenu.Item onclick={switchToOriginal}>
								{#if userMode !== "transcode"}
									<span class="mr-1">✓</span>
								{:else}
									<span class="mr-1 opacity-0">✓</span>
								{/if}
								Original{probeInfo?.height ? ` (${probeInfo.height}p)` : ""}
							</DropdownMenu.Item>
							{#each lowerQualities as height (height)}
								<DropdownMenu.Item onclick={() => switchToTranscode(height)}>
									{#if userMode === "transcode" && userPickedHeight === height}
										<span class="mr-1">✓</span>
									{:else}
										<span class="mr-1 opacity-0">✓</span>
									{/if}
									{height}p
								</DropdownMenu.Item>
							{/each}
						{:else}
							<DropdownMenu.Item onclick={() => pickQuality(-1)}>
								{#if selectedQuality < 0}
									<span class="mr-1">✓</span>
								{:else}
									<span class="mr-1 opacity-0">✓</span>
								{/if}
								Auto
							</DropdownMenu.Item>
							{#each availableQualities as level (level.index)}
								<DropdownMenu.Item onclick={() => pickQuality(level.index)}>
									{#if selectedQuality === level.index}
										<span class="mr-1">✓</span>
									{:else}
										<span class="mr-1 opacity-0">✓</span>
									{/if}
									{qualityLabel(level)}
								</DropdownMenu.Item>
							{/each}
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}

			<div class="flex items-center gap-1" style="--slider-track: rgb(255 255 255 / 0.2)">
				<Button variant="ghost" size="icon-touch" class="text-white/80 hover:bg-transparent hover:text-white"
					onclick={toggleMute}
					aria-label={muted || volume === 0 ? "Unmute" : "Mute"}
				>
					{#if muted || volume === 0}
						<VolumeXIcon class="size-5" />
					{:else}
						<Volume2Icon class="size-5" />
					{/if}
				</Button>
				<input
					type="range"
					min="0"
					max="1"
					step="0.05"
					value={muted ? 0 : volume}
					oninput={handleVolumeInput}
					aria-label="Volume"
					class="volume-slider hidden h-11 w-16 cursor-pointer appearance-none rounded-full bg-transparent md:block"
				/>
			</div>

			<Button variant="ghost" size="icon-touch" class="text-white/80 hover:bg-transparent hover:text-white"
				onclick={toggleFullscreen}
				aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
			>
				{#if isFullscreen}
					<MinimizeIcon class="size-5" />
				{:else}
					<MaximizeIcon class="size-5" />
				{/if}
			</Button>
		</div>
	</div>
	{/if}
	{/if}
</div>
