<script lang="ts">
	import { getPreviewUrl } from "$lib/preview.js";
	import { formatMediaTime } from "$lib/utils/format.js";
	import PlayIcon from "@lucide/svelte/icons/play";
	import PauseIcon from "@lucide/svelte/icons/pause";
	import Volume2Icon from "@lucide/svelte/icons/volume-2";
	import VolumeXIcon from "@lucide/svelte/icons/volume-x";

	let { path, url }: { path: string; url?: string } = $props();

	let audioEl = $state<HTMLAudioElement | null>(null);
	let playing = $state(false);
	let currentTime = $state(0);
	let duration = $state(0);
	let volume = $state(1);
	let muted = $state(false);
	let failed = $state(false);

	// Scrub state — same UI-layer coalescing pattern as VideoPreview.
	// Drag motion updates UI only; commit on `change` (pointerup/Enter/blur).
	let scrubbing = $state(false);
	let scrubTime = $state(0);

	function togglePlay() {
		if (!audioEl || failed) return;
		if (audioEl.paused) audioEl.play().catch(() => { failed = true; });
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
		scrubbing = true;
		scrubTime = Number((e.target as HTMLInputElement).value);
	}

	function handleSeekChange(e: Event) {
		if (!audioEl) return;
		const target = Number((e.target as HTMLInputElement).value);
		audioEl.currentTime = target;
		currentTime = target;
		scrubTime = target;
		scrubbing = false;
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
		if (tag === "BUTTON" && (e.key === " " || e.key === "Enter")) return;

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

	const displayTime = $derived(scrubbing ? scrubTime : currentTime);
	const seekPercent = $derived(duration > 0 ? (displayTime / duration) * 100 : 0);
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-1 items-center justify-center">
	<div class="w-full max-w-md rounded-xl border border-border bg-card p-6" data-preview-content>
		<audio
			bind:this={audioEl}
			src={url ?? getPreviewUrl(path)}
			preload="metadata"
			onplay={() => { playing = true; }}
			onpause={() => { playing = false; }}
			ontimeupdate={() => { if (audioEl) currentTime = audioEl.currentTime; }}
			onloadedmetadata={() => { if (audioEl) duration = audioEl.duration; }}
			onvolumechange={() => { if (audioEl) { volume = audioEl.volume; muted = audioEl.muted; } }}
			onended={() => { playing = false; }}
			onerror={() => { failed = true; }}
			class="hidden"
		></audio>

		{#if failed}
			<p class="text-center text-[15px] text-muted-foreground">Unable to play audio</p>
		{:else}
			<div class="flex flex-col gap-4">
				<div class="flex items-center justify-center">
					<button
						class="flex size-14 items-center justify-center rounded-full bg-accent-brand text-accent-brand-foreground transition-colors hover:bg-accent-brand/90"
						onclick={togglePlay}
					>
						{#if playing}
							<PauseIcon class="size-6" />
						{:else}
							<PlayIcon class="size-6 translate-x-0.5" />
						{/if}
					</button>
				</div>

				<div class="flex flex-col gap-1.5">
					<div class="seek-bar relative h-1.5 w-full cursor-pointer rounded-full bg-muted">
						<div
							class="absolute left-0 top-0 h-full rounded-full bg-accent-brand"
							style="width: {seekPercent}%"
						></div>
						<input
							type="range"
							min="0"
							max={duration}
							step="0.1"
							value={displayTime}
							oninput={handleSeekInput}
							onchange={handleSeekChange}
							class="absolute inset-x-0 -top-5 h-[calc(100%+2.5rem)] w-full cursor-pointer opacity-0"
						/>
					</div>
					<div class="flex justify-between font-mono text-meta text-muted-foreground">
						<span>{formatMediaTime(displayTime)}</span>
						<span>{formatMediaTime(duration)}</span>
					</div>
				</div>

				<div class="flex items-center gap-2">
					<button
						class="flex min-h-[44px] min-w-[44px] items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground"
						onclick={toggleMute}
						aria-label={muted || volume === 0 ? "Unmute" : "Mute"}
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
						aria-label="Volume"
						class="volume-slider h-11 w-full cursor-pointer appearance-none rounded-full bg-transparent"
					/>
				</div>
			</div>
		{/if}
	</div>
</div>

<style>
	.seek-bar:has(input:focus-visible) {
		outline: 2px solid var(--accent-brand);
		outline-offset: 2px;
		border-radius: 9999px;
	}
	.seek-bar input:focus-visible {
		outline: none;
	}
	.volume-slider::-webkit-slider-thumb {
		-webkit-appearance: none;
		appearance: none;
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: oklch(0.985 0 0);
		cursor: pointer;
	}
	.volume-slider::-webkit-slider-runnable-track {
		height: 4px;
		border-radius: 9999px;
		background: var(--color-muted);
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
		height: 4px;
		border-radius: 9999px;
		background: var(--color-muted);
	}
	.volume-slider:focus-visible {
		outline: 2px solid var(--accent-brand);
		outline-offset: 2px;
	}
</style>
