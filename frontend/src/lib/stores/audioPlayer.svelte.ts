import { getPreviewUrl } from "$lib/preview.js";

class AudioPlayerState {
	path = $state<string | null>(null);
	name = $state("");
	url = $state("");

	playing = $state(false);
	currentTime = $state(0);
	duration = $state(0);
	volume = $state(1);
	muted = $state(false);
	failed = $state(false);

	scrubbing = $state(false);
	scrubTime = $state(0);

	visible = $derived(this.path !== null);
	displayTime = $derived(this.scrubbing ? this.scrubTime : this.currentTime);
	progress = $derived(this.duration > 0 ? this.displayTime / this.duration : 0);

	load(path: string, name: string) {
		if (this.path === path) return;
		this.path = path;
		this.name = name;
		this.url = getPreviewUrl(path);

		this.playing = false;
		this.currentTime = 0;
		this.duration = 0;
		this.failed = false;
		this.scrubbing = false;
		this.scrubTime = 0;
	}

	close() {
		this.path = null;
		this.name = "";
		this.url = "";
		this.playing = false;
		this.currentTime = 0;
		this.duration = 0;
		this.failed = false;
		this.scrubbing = false;
		this.scrubTime = 0;
	}
}

export const audioPlayer = new AudioPlayerState();
