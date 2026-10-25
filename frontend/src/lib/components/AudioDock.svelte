<script lang="ts">
	import { untrack } from "svelte";
	import { audioPlayer } from "$lib/stores/audioPlayer.svelte.js";
	import { formatMediaTime } from "$lib/utils/format.js";
	import PlayIcon from "@lucide/svelte/icons/play";
	import PauseIcon from "@lucide/svelte/icons/pause";
	import Volume2Icon from "@lucide/svelte/icons/volume-2";
	import Volume1Icon from "@lucide/svelte/icons/volume-1";
	import VolumeXIcon from "@lucide/svelte/icons/volume-x";
	import XIcon from "@lucide/svelte/icons/x";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";

	let audioEl = $state<HTMLAudioElement | null>(null);
	let volumeOpen = $state(false);
	let volumePopupEl = $state<HTMLDivElement | null>(null);
	let volumeBtnEl = $state<HTMLButtonElement | null>(null);

	// AbortError (pause or track change) and NotAllowedError (autoplay blocked) aren't failures
	function playOrFail() {
		audioEl?.play().catch((e) => {
			if (e?.name !== "AbortError" && e?.name !== "NotAllowedError") audioPlayer.failed = true;
		});
	}

	function togglePlay() {
		if (!audioEl || audioPlayer.failed) return;
		if (audioEl.paused) playOrFail();
		else audioEl.pause();
	}

	function retry() {
		if (!audioEl) return;
		audioPlayer.failed = false;
		audioEl.load();
		playOrFail();
	}

	const VOLUME_STEP = 0.05;

	function setVolume(v: number) {
		if (!audioEl) return;
		audioEl.volume = Math.max(0, Math.min(1, v));
		if (audioEl.volume > 0 && audioEl.muted) audioEl.muted = false;
	}

	function handleVolumeKeydown(e: KeyboardEvent) {
		if (!audioEl) return;
		let next: number | null = null;
		// Step from what the slider shows (0 while muted), read live since volumechange lags
		const shown = audioEl.muted ? 0 : audioEl.volume;
		if (e.key === "ArrowUp" || e.key === "ArrowRight") next = shown + VOLUME_STEP;
		else if (e.key === "ArrowDown" || e.key === "ArrowLeft") next = shown - VOLUME_STEP;
		else if (e.key === "Home") next = 0;
		else if (e.key === "End") next = 1;
		if (next === null) return;
		e.preventDefault();
		// Keep the window handler from treating the arrows as seeks
		e.stopPropagation();
		setVolume(next);
	}

	function toggleMute() {
		if (!audioEl) return;
		audioEl.muted = !audioEl.muted;
	}

	function seek(offset: number) {
		if (!audioEl) return;
		audioEl.currentTime = Math.max(0, Math.min(audioPlayer.duration, audioEl.currentTime + offset));
	}

	function handleSeekDown(e: PointerEvent) {
		e.preventDefault();
		const bar = e.currentTarget as HTMLElement;
		bar.setPointerCapture(e.pointerId);

		function setTimeFromPointer(ev: PointerEvent) {
			const rect = bar.getBoundingClientRect();
			const ratio = Math.max(0, Math.min(1, (ev.clientX - rect.left) / rect.width));
			const time = ratio * audioPlayer.duration;
			audioPlayer.scrubbing = true;
			audioPlayer.scrubTime = time;
		}

		setTimeFromPointer(e);

		function onMove(ev: PointerEvent) { setTimeFromPointer(ev); }
		function onUp(ev: PointerEvent) {
			const rect = bar.getBoundingClientRect();
			const ratio = Math.max(0, Math.min(1, (ev.clientX - rect.left) / rect.width));
			const time = ratio * audioPlayer.duration;
			if (audioEl) {
				audioEl.currentTime = time;
				audioPlayer.currentTime = time;
			}
			audioPlayer.scrubbing = false;
			bar.removeEventListener("pointermove", onMove);
			bar.removeEventListener("pointerup", onUp);
		}

		bar.addEventListener("pointermove", onMove);
		bar.addEventListener("pointerup", onUp);
	}

	function handleVolumeTrackDown(e: PointerEvent) {
		e.preventDefault();
		const track = e.currentTarget as HTMLElement;
		track.setPointerCapture(e.pointerId);

		function setVolumeFromPointer(ev: PointerEvent) {
			const rect = track.getBoundingClientRect();
			const ratio = 1 - Math.max(0, Math.min(1, (ev.clientY - rect.top) / rect.height));
			if (audioEl) {
				audioEl.volume = ratio;
				audioPlayer.volume = ratio;
				if (ratio > 0 && audioPlayer.muted) audioEl.muted = false;
			}
		}

		setVolumeFromPointer(e);

		function onMove(ev: PointerEvent) { setVolumeFromPointer(ev); }
		function onUp() {
			track.removeEventListener("pointermove", onMove);
			track.removeEventListener("pointerup", onUp);
		}

		track.addEventListener("pointermove", onMove);
		track.addEventListener("pointerup", onUp);
	}

	function handleVolumeClick() {
		volumeOpen = !volumeOpen;
	}

	function handleWindowClick(e: MouseEvent) {
		if (!volumeOpen) return;
		const target = e.target as Node;
		if (volumePopupEl?.contains(target) || volumeBtnEl?.contains(target)) return;
		volumeOpen = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!audioPlayer.visible) return;
		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
		if (tag === "BUTTON" && (e.key === " " || e.key === "Enter")) return;
		if (document.querySelector("dialog[open]")) return;
		if ((e.target as HTMLElement)?.closest('[role="menu"], [role="listbox"], [role="tablist"], [role="tree"]')) return;

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
		}
	}

	$effect(() => {
		const el = audioEl;
		if (!el) return;
		// Untracked: a volume change must not rerun this, since the cleanup pauses
		untrack(() => {
			el.volume = audioPlayer.volume;
			el.muted = audioPlayer.muted;
		});
		return () => { el.pause(); };
	});

	let seenReloads = audioPlayer.reloads;
	$effect(() => {
		const n = audioPlayer.reloads;
		if (n === seenReloads) return;
		seenReloads = n;
		untrack(retry);
	});

	const seekPercent = $derived(audioPlayer.duration > 0 ? (audioPlayer.displayTime / audioPlayer.duration) * 100 : 0);
	const displayVolume = $derived(audioPlayer.muted ? 0 : audioPlayer.volume);
</script>

<svelte:window onkeydown={handleKeydown} onclick={handleWindowClick} />

{#if audioPlayer.visible}
	<div class="fixed inset-x-0 bottom-0 z-40 flex flex-col border-t border-border-2 bg-card">
		<audio
			bind:this={audioEl}
			src={audioPlayer.url}
			preload="metadata"
			onplay={() => { audioPlayer.playing = true; }}
			onpause={() => { audioPlayer.playing = false; }}
			ontimeupdate={() => { if (audioEl) audioPlayer.currentTime = audioEl.currentTime; }}
			onloadedmetadata={() => { if (audioEl) audioPlayer.duration = audioEl.duration; }}
			onvolumechange={() => { if (audioEl) { audioPlayer.volume = audioEl.volume; audioPlayer.muted = audioEl.muted; } }}
			onended={() => { audioPlayer.playing = false; }}
			onerror={() => { audioPlayer.failed = true; }}
			class="hidden"
		></audio>

		<!-- Seek bar -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="group relative flex h-5 w-full cursor-pointer items-center px-0 outline-none focus-visible:ring-2 focus-visible:ring-ring"
			role="slider"
			tabindex={0}
			aria-label="Seek"
			aria-valuemin={0}
			aria-valuemax={Math.round(audioPlayer.duration)}
			aria-valuenow={Math.round(audioPlayer.displayTime)}
			onpointerdown={handleSeekDown}
		>
			<div class="relative h-1 w-full rounded-full bg-muted transition-[height] group-hover:h-1.5">
				<div
					class="absolute left-0 top-0 h-full rounded-full bg-accent-brand"
					style="width: {seekPercent}%"
				></div>
			</div>
			<div
				class="absolute size-3.5 -translate-x-1/2 rounded-full bg-white opacity-0 transition-opacity group-hover:opacity-100"
				style="left: {seekPercent}%"
			></div>
		</div>

		<!-- Controls row -->
		<div class="flex h-14 items-center gap-4 px-4">
			<!-- Play / Pause -->
			<button
				class="flex size-10 shrink-0 items-center justify-center rounded-full bg-accent-brand text-accent-brand-foreground transition-[filter] hover:brightness-110"
				onclick={togglePlay}
				aria-label={audioPlayer.playing ? "Pause" : "Play"}
			>
				{#if audioPlayer.playing}
					<PauseIcon class="size-5" />
				{:else}
					<PlayIcon class="size-5 translate-x-0.5" />
				{/if}
			</button>

			<!-- Filename -->
			<span class="min-w-0 flex-1 truncate text-[15px] font-medium text-foreground">
				{audioPlayer.name}
			</span>

			<!-- Time -->
			{#if audioPlayer.failed}
				<span class="shrink-0 text-meta text-destructive">Unable to play audio</span>
				<button
					class="flex h-9 shrink-0 items-center gap-1.5 rounded-full px-3 text-meta text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
					onclick={retry}
				>
					<RotateCcwIcon class="size-3.5" />
					Retry
				</button>
			{:else}
				<span class="shrink-0 font-mono text-sm text-muted-foreground">
					{formatMediaTime(audioPlayer.displayTime)} / {formatMediaTime(audioPlayer.duration)}
				</span>
			{/if}

			<!-- Volume (desktop only) -->
			<div class="relative hidden md:block">
				<button
					bind:this={volumeBtnEl}
					class="flex size-10 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted"
					onclick={handleVolumeClick}
					aria-label="Volume"
					aria-expanded={volumeOpen}
				>
					{#if audioPlayer.muted || audioPlayer.volume === 0}
						<VolumeXIcon class="size-5" />
					{:else if audioPlayer.volume < 0.5}
						<Volume1Icon class="size-5" />
					{:else}
						<Volume2Icon class="size-5" />
					{/if}
				</button>

				{#if volumeOpen}
					<div
						bind:this={volumePopupEl}
						class="absolute bottom-full left-1/2 mb-2 flex -translate-x-1/2 flex-col items-center rounded-lg border border-border-2 bg-card px-2 py-3"
					>
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div
							class="relative h-24 w-1 cursor-pointer rounded-full bg-muted outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card"
							role="slider"
							tabindex={0}
							aria-label="Volume"
							aria-valuemin={0}
							aria-valuemax={100}
							aria-valuenow={Math.round(displayVolume * 100)}
							onpointerdown={handleVolumeTrackDown}
							onkeydown={handleVolumeKeydown}
						>
							<div
								class="absolute inset-x-0 bottom-0 rounded-full bg-foreground"
								style="height: {displayVolume * 100}%"
							></div>
							<div
								class="absolute left-1/2 size-3 -translate-x-1/2 translate-y-1/2 rounded-full bg-white"
								style="bottom: {displayVolume * 100}%"
							></div>
						</div>
						<button
							class="mt-3 flex size-7 items-center justify-center rounded-full text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
							onclick={toggleMute}
							aria-label={audioPlayer.muted || audioPlayer.volume === 0 ? "Unmute" : "Mute"}
						>
							{#if audioPlayer.muted || audioPlayer.volume === 0}
								<VolumeXIcon class="size-4" />
							{:else}
								<Volume2Icon class="size-4" />
							{/if}
						</button>
					</div>
				{/if}
			</div>

			<!-- Close -->
			<button
				class="flex size-10 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted"
				onclick={() => audioPlayer.close()}
				aria-label="Close audio player"
			>
				<XIcon class="size-5" />
			</button>
		</div>
	</div>
{/if}
