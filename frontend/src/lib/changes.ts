import { request } from "$lib/api";

const POLL_INTERVAL_MS = 5_000;
const ERROR_BACKOFF_MS = [5_000, 15_000, 30_000, 60_000];

interface ChangesEvent {
	id: number;
	type: string;
	payload: any;
	createdAt: number;
}

interface ChangesResponse {
	cursor: number;
	events: ChangesEvent[];
	behind?: boolean;
}

type Listener = (payload: any) => void;
type BehindListener = () => void;

// null cursor = bootstrap mode: first request omits `since` so the server
// returns the current latest id with no event payload.
let cursor: number | null = null;
let timer: ReturnType<typeof setTimeout> | null = null;
let polling = false;
let visListenerAttached = false;
let inflight = false;
let errorCount = 0;

const listeners = new Map<string, Set<Listener>>();
const behindListeners = new Set<BehindListener>();

async function tick(): Promise<void> {
	if (!polling || inflight || document.hidden) return;
	inflight = true;
	try {
		const path = cursor === null ? "/api/changes" : "/api/changes?since=" + cursor;
		const res = await request<ChangesResponse>("GET", path);
		cursor = res.cursor;
		errorCount = 0;
		for (const ev of res.events ?? []) {
			const subs = listeners.get(ev.type);
			if (!subs) continue;
			for (const fn of subs) {
				try {
					fn(ev.payload);
				} catch (e) {
					console.error("changes listener error", e);
				}
			}
		}
		if (res.behind) {
			for (const fn of behindListeners) {
				try {
					fn();
				} catch (e) {
					console.error("changes onBehind listener error", e);
				}
			}
		}
		schedule(POLL_INTERVAL_MS);
	} catch {
		// Don't reset cursor — resume from the same point once the network recovers.
		const idx = Math.min(errorCount, ERROR_BACKOFF_MS.length - 1);
		errorCount++;
		schedule(ERROR_BACKOFF_MS[idx]);
	} finally {
		inflight = false;
	}
}

function schedule(delay: number) {
	if (!polling) return;
	if (timer) clearTimeout(timer);
	timer = setTimeout(tick, delay);
}

function onVisibilityChange() {
	if (!polling) return;
	if (document.hidden) {
		if (timer) {
			clearTimeout(timer);
			timer = null;
		}
	} else {
		schedule(0);
	}
}

export const changes = {
	start(): void {
		if (polling) return;
		polling = true;
		if (!visListenerAttached) {
			document.addEventListener("visibilitychange", onVisibilityChange);
			visListenerAttached = true;
		}
		schedule(0);
	},

	stop(): void {
		if (!polling) return;
		polling = false;
		if (timer) {
			clearTimeout(timer);
			timer = null;
		}
		if (visListenerAttached) {
			document.removeEventListener("visibilitychange", onVisibilityChange);
			visListenerAttached = false;
		}
	},

	on(type: string, fn: Listener): () => void {
		let set = listeners.get(type);
		if (!set) {
			set = new Set();
			listeners.set(type, set);
		}
		set.add(fn);
		return () => {
			const s = listeners.get(type);
			if (!s) return;
			s.delete(fn);
			if (s.size === 0) listeners.delete(type);
		};
	},

	onBehind(fn: BehindListener): () => void {
		behindListeners.add(fn);
		return () => {
			behindListeners.delete(fn);
		};
	},
};
