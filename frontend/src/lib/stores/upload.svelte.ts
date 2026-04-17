export interface UploadItem {
	id: string;
	name: string;
	size: number;
	progress: number;
	status: "pending" | "uploading" | "complete" | "error";
	error?: string;
}

class UploadState {
	items = $state<UploadItem[]>([]);
	minimized = $state(false);
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

	get totalProgress() {
		if (this.items.length === 0) return 0;
		const total = this.items.reduce((sum, i) => sum + i.progress, 0);
		return Math.round(total / this.items.length);
	}

	addFile(id: string, name: string, size: number) {
		this.items.push({ id, name, size, progress: 0, status: "pending" });
		this.minimized = false;
		this.clearAutoMinimize();
	}

	updateProgress(id: string, progress: number) {
		const item = this.items.find((i) => i.id === id);
		if (item) {
			item.progress = progress;
			item.status = "uploading";
		}
	}

	markComplete(id: string) {
		const item = this.items.find((i) => i.id === id);
		if (item) {
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
		if (this.items.length === 0) this.minimized = false;
	}

	clear() {
		this.items = [];
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
