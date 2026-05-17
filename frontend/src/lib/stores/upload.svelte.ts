export interface UploadItem {
	id: string;
	name: string;
	size: number;
	progress: number;
	bytesUploaded: number;
	status: "pending" | "uploading" | "complete" | "error";
	error?: string;
	group?: string;
}

export interface GroupMeta {
	name: string;
	targetDir: string;
}

class UploadState {
	items = $state<UploadItem[]>([]);
	groupMeta = $state<Record<string, GroupMeta>>({});
	minimized = $state(false);
	speed = $state(0);
	eta = $state<number | null>(null);
	private autoMinimizeTimer: ReturnType<typeof setTimeout> | null = null;

	get hasItems() {
		return this.items.length > 0;
	}

	get activeCount() {
		return this.items.filter((i) => i.status === "uploading" || i.status === "pending").length;
	}

	get isComplete() {
		return this.items.length > 0 && this.activeCount === 0;
	}

	get totalBytes() {
		return this.items.reduce((sum, i) => sum + i.size, 0);
	}

	get totalBytesUploaded() {
		return this.items.reduce((sum, i) => sum + i.bytesUploaded, 0);
	}

	get totalProgress() {
		const total = this.totalBytes;
		if (total === 0) return 0;
		return Math.round((this.totalBytesUploaded / total) * 100);
	}

	addFile(id: string, name: string, size: number) {
		this.items.push({ id, name, size, progress: 0, bytesUploaded: 0, status: "pending" });
		this.minimized = false;
		this.clearAutoMinimize();
	}

	addFiles(files: { id: string; name: string; size: number }[], group?: string) {
		this.items = [...this.items, ...files.map((f) => ({ ...f, progress: 0, bytesUploaded: 0, status: "pending" as const, group }))];
		this.minimized = false;
		this.clearAutoMinimize();
	}

	addGroup(groupId: string, name: string, targetDir: string) {
		this.groupMeta = { ...this.groupMeta, [groupId]: { name, targetDir } };
	}

	removeGroup(groupId: string) {
		this.items = this.items.filter((i) => i.group !== groupId);
		const { [groupId]: _, ...rest } = this.groupMeta;
		this.groupMeta = rest;
	}

	updateProgress(id: string, bytesUploaded: number) {
		const item = this.items.find((i) => i.id === id);
		if (item) {
			item.bytesUploaded = bytesUploaded;
			item.progress = item.size > 0 ? Math.round((bytesUploaded / item.size) * 100) : 0;
			item.status = "uploading";
		}
	}

	updateSpeedAndEta(speed: number, eta: number | null) {
		this.speed = speed;
		this.eta = eta;
	}

	markComplete(id: string) {
		const item = this.items.find((i) => i.id === id);
		if (item) {
			item.bytesUploaded = item.size;
			item.progress = 100;
			item.status = "complete";
		}
		this.checkAutoMinimize();
	}

	markError(id: string, error: string) {
		const item = this.items.find((i) => i.id === id);
		if (item) {
			item.status = "error";
			item.error = error;
		}
	}

	removeFile(id: string) {
		this.items = this.items.filter((i) => i.id !== id);
	}

	clearCompleted() {
		this.items = this.items.filter((i) => i.status !== "complete");
		// Clean up groups with no remaining items
		const activeGroups = new Set(this.items.map((i) => i.group).filter(Boolean));
		const newMeta: Record<string, GroupMeta> = {};
		for (const [id, meta] of Object.entries(this.groupMeta)) {
			if (activeGroups.has(id)) newMeta[id] = meta;
		}
		this.groupMeta = newMeta;
		if (this.items.length === 0) this.minimized = false;
	}

	clear() {
		this.items = [];
		this.groupMeta = {};
		this.speed = 0;
		this.eta = null;
		this.minimized = false;
		this.clearAutoMinimize();
	}

	private checkAutoMinimize() {
		if (this.isComplete) {
			this.autoMinimizeTimer = setTimeout(() => {
				this.minimized = true;
			}, 3000);
		}
	}

	private clearAutoMinimize() {
		if (this.autoMinimizeTimer) {
			clearTimeout(this.autoMinimizeTimer);
			this.autoMinimizeTimer = null;
		}
	}
}

export const uploadState = new UploadState();
