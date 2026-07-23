type Listener = (payload: any) => void;
type BehindListener = () => void;

let es: EventSource | null = null;
let active = false;

const listeners = new Map<string, Set<Listener>>();
const behindListeners = new Set<BehindListener>();

function connect() {
	if (es) return;
	es = new EventSource("/api/changes");
	es.onerror = () => {
		if (es?.readyState === EventSource.CLOSED) {
			disconnect();
			if (active) setTimeout(connect, 5000);
		}
	};
	es.onmessage = (e) => {
		let msg: { type: string; payload?: any };
		try {
			msg = JSON.parse(e.data);
		} catch {
			return;
		}
		if (msg.type === "behind") {
			for (const fn of behindListeners) {
				try {
					fn();
				} catch (err) {
					console.error("changes onBehind listener error", err);
				}
			}
			return;
		}
		const subs = listeners.get(msg.type);
		if (!subs) return;
		for (const fn of subs) {
			try {
				fn(msg.payload);
			} catch (err) {
				console.error("changes listener error", err);
			}
		}
	};
}

function disconnect() {
	if (!es) return;
	es.close();
	es = null;
}

export const changes = {
	start(): void {
		if (active) return;
		active = true;
		connect();
	},

	stop(): void {
		if (!active) return;
		active = false;
		disconnect();
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
