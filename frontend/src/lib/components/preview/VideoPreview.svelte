<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { formatMediaTime } from "$lib/utils/format.js";
	import type { FileInfo } from "$lib/types";
	import PlayIcon from "@lucide/svelte/icons/play";
	import PauseIcon from "@lucide/svelte/icons/pause";
	import Volume2Icon from "@lucide/svelte/icons/volume-2";
	import VolumeXIcon from "@lucide/svelte/icons/volume-x";
	import MaximizeIcon from "@lucide/svelte/icons/maximize";

	let { file }: { file: FileInfo } = $props();

	let videoEl = $state<HTMLVideoElement | null>(null);
	let playing = $state(false);
	let currentTime = $state(0);
	let duration = $state(0);
	let volume = $state(1);
	let muted = $state(false);
	let bufferedEnd = $state(0);
	let showControls = $state(true);
	let controlsTimer: ReturnType<typeof setTimeout> | null = null;
	let lastSaveTime = 0;

	const STORAGE_PREFIX = "onyx-video-pos:";

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

	function togglePlay() {
		if (!videoEl) return;
		if (videoEl.paused) videoEl.play();
		else videoEl.pause();
	}

	function toggleMute() {
		if (!videoEl) return;
		videoEl.muted = !videoEl.muted;
	}

	function toggleFullscreen() {
		if (!videoEl) return;
		if (document.fullscreenElement) document.exitFullscreen();
		else videoEl.requestFullscreen();
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

	$effect(() => {
		const el = videoEl;
		if (!el) return;
		restorePosition();
		return () => {
			el.pause();
			if (controlsTimer) clearTimeout(controlsTimer);
		};
	});

	const seekPercent = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);
	const bufferedPercent = $derived(duration > 0 ? (bufferedEnd / duration) * 100 : 0);
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	class="group relative flex flex-1 items-center justify-center overflow-hidden"
	onmousemove={resetControlsTimer}
	onmouseleave={() => { if (playing) showControls = false; }}
>
	<!-- svelte-ignore a11y_media_has_caption -->
	<video
		bind:this={videoEl}
		src={getPreviewUrl(file.path)}
		class="max-h-full max-w-full"
		preload="metadata"
		onclick={togglePlay}
		ondblclick={toggleFullscreen}
		onplay={() => { playing = true; resetControlsTimer(); }}
		onpause={() => { playing = false; showControls = true; if (controlsTimer) clearTimeout(controlsTimer); }}
		ontimeupdate={handleTimeUpdate}
		onloadedmetadata={() => { if (videoEl) duration = videoEl.duration; }}
		onvolumechange={() => { if (videoEl) { volume = videoEl.volume; muted = videoEl.muted; } }}
		onended={() => { playing = false; showControls = true; clearPosition(); }}
	></video>

	{#if !playing && currentTime === 0}
		<button
			class="absolute inset-0 flex items-center justify-center"
			onclick={togglePlay}
		>
			<div class="flex size-16 items-center justify-center rounded-full bg-black/60 text-white">
				<PlayIcon class="size-8 translate-x-0.5" />
			</div>
		</button>
	{/if}

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

			<button
				class="rounded p-1 text-white/80 transition-colors hover:text-white"
				onclick={toggleFullscreen}
			>
				<MaximizeIcon class="size-4" />
			</button>
		</div>
	</div>
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
