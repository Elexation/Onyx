<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
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
	import SettingsIcon from "@lucide/svelte/icons/settings";

	let { file, onclose, url }: { file: FileInfo; onclose: () => void; url?: string } = $props();

	type PlaybackMode = "loading" | "native" | "transcode-required" | "no-video";

	let videoEl = $state<HTMLVideoElement | null>(null);
	let containerEl = $state<HTMLDivElement | null>(null);
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

	let detectedMode = $state<PlaybackMode>("loading");
	let nativeSupported = $state(false);
	let userMode = $state<"original" | "transcode" | null>(null);
	let userPickedHeight = $state<number | null>(null);
	let pendingSeek = $state<number | null>(null);

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

	const seekPercent = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);
	const bufferedPercent = $derived(duration > 0 ? (bufferedEnd / duration) * 100 : 0);

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

	function togglePlay() {
		if (!videoEl || failed) return;
		if (videoEl.paused) videoEl.play().catch(() => { failed = true; });
		else videoEl.pause();
	}

	function toggleMute() {
		if (!videoEl) return;
		videoEl.muted = !videoEl.muted;
	}

	let clickTimer: ReturnType<typeof setTimeout> | null = null;

	function handleVideoClick() {
		if (clickTimer) {
			clearTimeout(clickTimer);
			clickTimer = null;
			return;
		}
		clickTimer = setTimeout(() => {
			clickTimer = null;
			togglePlay();
		}, 200);
	}

	function handleVideoDblClick() {
		if (clickTimer) {
			clearTimeout(clickTimer);
			clickTimer = null;
		}
		toggleFullscreen();
	}

	function toggleFullscreen() {
		if (!containerEl) return;
		if (document.fullscreenElement) document.exitFullscreen();
		else containerEl.requestFullscreen();
	}

	function seek(offset: number) {
		if (!videoEl) return;
		videoEl.currentTime = Math.max(0, Math.min(duration, videoEl.currentTime + offset));
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
		if (!videoEl) return;
		videoEl.currentTime = Number((e.target as HTMLInputElement).value);
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

	function handleKeydown(e: KeyboardEvent) {
		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

		switch (e.key) {
			case " ":
				e.preventDefault();
				togglePlay();
				break;
			case "ArrowLeft":
				e.preventDefault();
				seek(-10);
				break;
			case "ArrowRight":
				e.preventDefault();
				seek(10);
				break;
			case "ArrowUp":
				e.preventDefault();
				if (videoEl) {
					videoEl.volume = Math.min(1, volume + 0.1);
					volume = videoEl.volume;
				}
				break;
			case "ArrowDown":
				e.preventDefault();
				if (videoEl) {
					videoEl.volume = Math.max(0, volume - 0.1);
					volume = videoEl.volume;
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
		pendingSeek = videoEl.currentTime;
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

		pendingSeek = videoEl.currentTime;
		userMode = "transcode";
		failed = false;
	}

	// --- Effects ---

	$effect(() => {
		const el = videoEl;
		if (!el) return;
		restoreVolume();
		return () => {
			el.pause();
			if (controlsTimer) clearTimeout(controlsTimer);
			if (clickTimer) clearTimeout(clickTimer);
		};
	});

	$effect(() => {
		getSettings()
			.then((s) => {
				const raw = s["playback.default_quality_ceiling"];
				const n = raw ? parseInt(raw, 10) : NaN;
				if (!isNaN(n)) {
					defaultQualityCeiling = n;
					hlsHandle?.setAutoLevelCap(n);
				}
			})
			.catch(() => { /* default stays 1080 */ });
	});

	$effect(() => {
		const path = file.path;
		if (url) {
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
		fetchProbeInfo(path).then(async (result) => {
			if (path !== file.path) return;
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
			if (path !== file.path) return;
			nativeSupported = native;
			detectedMode = native ? "native" : "transcode-required";
		});
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

		const masterUrl = `/api/stream/master${filePath}`;
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

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	bind:this={containerEl}
	class="group relative flex flex-1 items-center justify-center overflow-hidden bg-black"
	onmousemove={resetControlsTimer}
	onmouseleave={() => { if (playing) showControls = false; }}
>
	{#if playback === "loading"}
		<p class="text-sm text-muted-foreground" data-preview-content>Loading…</p>
	{:else if playback === "no-video"}
		<p class="text-sm text-muted-foreground" data-preview-content>No playable video stream in this file.</p>
	{:else}
	<!-- svelte-ignore a11y_media_has_caption -->
	<video
		bind:this={videoEl}
		class="h-full w-full object-contain"
		preload="metadata"
		data-preview-content
		onclick={handleVideoClick}
		ondblclick={handleVideoDblClick}
		onplay={() => { playing = true; resetControlsTimer(); }}
		onpause={() => { playing = false; showControls = true; if (controlsTimer) clearTimeout(controlsTimer); }}
		ontimeupdate={handleTimeUpdate}
		onloadedmetadata={() => {
			if (videoEl) {
				duration = videoEl.duration;
				if (pendingSeek !== null && pendingSeek > 0) {
					videoEl.currentTime = pendingSeek;
					pendingSeek = null;
					videoEl.play().catch(() => { failed = true; });
				} else {
					restorePosition();
				}
			}
		}}
		onvolumechange={() => { if (videoEl) { volume = videoEl.volume; muted = videoEl.muted; saveVolume(); } }}
		onended={() => { playing = false; showControls = true; clearPosition(); }}
		onerror={() => { if (playback === "native") failed = true; }}
	></video>

	{#if failed}
		<button class="absolute inset-0 flex items-center justify-center" onclick={onclose}>
			<p class="text-sm text-muted-foreground">Unable to play video</p>
		</button>
	{:else}
		{#if !playing && currentTime === 0}
			<button
				class="absolute inset-0 flex items-center justify-center"
				onclick={togglePlay}
				data-preview-content
			>
				<div class="flex size-16 items-center justify-center rounded-full bg-black/60 text-white">
					<PlayIcon class="size-8 translate-x-0.5" />
				</div>
			</button>
		{/if}
	{/if}

	{#if !failed}
	<div
		class="absolute bottom-0 left-0 right-0 flex flex-col gap-1 bg-black/70 px-3 py-2 backdrop-blur-sm transition-opacity duration-200"
		class:opacity-0={!showControls}
		class:pointer-events-none={!showControls}
		onclick={(e) => e.stopPropagation()}
	>
		<div class="seek-bar relative h-1 w-full cursor-pointer rounded-full bg-white/20">
			<div
				class="absolute left-0 top-0 h-full rounded-full bg-white/30"
				style="width: {bufferedPercent}%"
			></div>
			<div
				class="absolute left-0 top-0 h-full rounded-full bg-white"
				style="width: {seekPercent}%"
			></div>
			<input
				type="range"
				min="0"
				max={duration}
				step="0.1"
				value={currentTime}
				oninput={handleSeekInput}
				class="absolute inset-0 h-full w-full cursor-pointer opacity-0"
			/>
		</div>

		<div class="flex items-center gap-2">
			<button
				class="rounded p-1 text-white/80 transition-colors hover:text-white"
				onclick={togglePlay}
			>
				{#if playing}
					<PauseIcon class="size-4" />
				{:else}
					<PlayIcon class="size-4" />
				{/if}
			</button>

			<span class="min-w-0 text-xs text-white/80">
				{formatMediaTime(currentTime)} / {formatMediaTime(duration)}
			</span>

			<div class="flex-1"></div>

			<div class="flex items-center gap-1">
				<button
					class="rounded p-1 text-white/80 transition-colors hover:text-white"
					onclick={toggleMute}
				>
					{#if muted || volume === 0}
						<VolumeXIcon class="size-4" />
					{:else}
						<Volume2Icon class="size-4" />
					{/if}
				</button>
				<input
					type="range"
					min="0"
					max="1"
					step="0.05"
					value={muted ? 0 : volume}
					oninput={handleVolumeInput}
					class="volume-slider h-1 w-16 cursor-pointer appearance-none rounded-full bg-white/20"
				/>
			</div>

			{#if showQualityMenu}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<button
								{...props}
								class="flex items-center gap-1 rounded p-1 text-xs text-white/80 transition-colors hover:text-white"
							>
								<SettingsIcon class="size-4" />
								<span class="hidden sm:inline">{qualityButtonLabel}</span>
							</button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="min-w-36">
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

			<button
				class="rounded p-1 text-white/80 transition-colors hover:text-white"
				onclick={toggleFullscreen}
			>
				<MaximizeIcon class="size-4" />
			</button>
		</div>
	</div>
	{/if}
	{/if}
</div>

<style>
	.volume-slider::-webkit-slider-thumb {
		-webkit-appearance: none;
		appearance: none;
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: white;
		cursor: pointer;
	}
	.volume-slider::-moz-range-thumb {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: white;
		border: none;
		cursor: pointer;
	}
	.volume-slider::-moz-range-track {
		background: transparent;
	}
</style>
