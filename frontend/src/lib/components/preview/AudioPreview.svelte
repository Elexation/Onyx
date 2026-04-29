<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { formatMediaTime } from "$lib/utils/format.js";
	import PlayIcon from "@lucide/svelte/icons/play";
	import PauseIcon from "@lucide/svelte/icons/pause";
	import Volume2Icon from "@lucide/svelte/icons/volume-2";
	import VolumeXIcon from "@lucide/svelte/icons/volume-x";

	let { path }: { path: string } = $props();

	let audioEl = $state<HTMLAudioElement | null>(null);
	let playing = $state(false);
	let currentTime = $state(0);
	let duration = $state(0);
	let volume = $state(1);
	let muted = $state(false);

	function togglePlay() {
		if (!audioEl) return;
		if (audioEl.paused) audioEl.play();
		else audioEl.pause();
	}

	function toggleMute() {
		if (!audioEl) return;
		audioEl.muted = !audioEl.muted;
	}

	function seek(offset: number) {
		if (!audioEl) return;
		audioEl.currentTime = Math.max(0, Math.min(duration, audioEl.currentTime + offset));
	}

	function handleSeekInput(e: Event) {
		if (!audioEl) return;
		audioEl.currentTime = Number((e.target as HTMLInputElement).value);
	}

	function handleVolumeInput(e: Event) {
		if (!audioEl) return;
		const v = Number((e.target as HTMLInputElement).value);
		audioEl.volume = v;
		volume = v;
		if (v > 0 && muted) audioEl.muted = false;
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
		}
	}

	$effect(() => {
		const el = audioEl;
		if (!el) return;
		return () => { el.pause(); };
	});

	const seekPercent = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-1 items-center justify-center">
	<div class="w-full max-w-md rounded-lg bg-card p-6">
		<audio
			bind:this={audioEl}
			src={getPreviewUrl(path)}
			preload="metadata"
			onplay={() => { playing = true; }}
			onpause={() => { playing = false; }}
			ontimeupdate={() => { if (audioEl) currentTime = audioEl.currentTime; }}
			onloadedmetadata={() => { if (audioEl) duration = audioEl.duration; }}
			onvolumechange={() => { if (audioEl) { volume = audioEl.volume; muted = audioEl.muted; } }}
			onended={() => { playing = false; }}
			class="hidden"
		></audio>

		<div class="flex flex-col gap-4">
			<div class="flex items-center justify-center">
				<button
					class="flex size-12 items-center justify-center rounded-full bg-accent text-foreground transition-colors hover:bg-accent/80"
					onclick={togglePlay}
				>
					{#if playing}
						<PauseIcon class="size-5" />
					{:else}
						<PlayIcon class="size-5 translate-x-0.5" />
					{/if}
				</button>
			</div>

			<div class="flex flex-col gap-1">
				<div class="seek-bar relative h-1.5 w-full cursor-pointer rounded-full bg-muted">
					<div
						class="absolute left-0 top-0 h-full rounded-full bg-foreground"
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
				<div class="flex justify-between text-xs text-muted-foreground">
					<span>{formatMediaTime(currentTime)}</span>
					<span>{formatMediaTime(duration)}</span>
				</div>
			</div>

			<div class="flex items-center gap-2">
				<button
					class="rounded p-1 text-muted-foreground transition-colors hover:text-foreground"
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
					class="volume-slider h-1 w-full cursor-pointer appearance-none rounded-full bg-muted"
				/>
			</div>
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
		background: oklch(0.985 0 0);
		cursor: pointer;
	}
	.volume-slider::-moz-range-thumb {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: oklch(0.985 0 0);
		border: none;
		cursor: pointer;
	}
	.volume-slider::-moz-range-track {
		background: transparent;
	}
</style>
