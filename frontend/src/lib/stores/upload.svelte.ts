// Loose (non-folder) files keep a per-file reactive row — these drops are small.
export interface LooseItem {
	id: string;
	name: string;
	size: number;
	bytesUploaded: number;
	progress: number;
	status: "pending" | "uploading" | "complete" | "error";
	error?: string;
}

// Folder uploads collapse to ONE aggregate per top-level folder. Counters and
// byte totals are maintained incrementally (O(1) per upload event) so a drop of
// tens of thousands of files never materializes tens of thousands of reactive
// rows — that was the source of the main-thread freeze.
export interface UploadGroup {
	id: string;
	name: string;
	targetDir: string;
	fileCount: number;
	completedCount: number;
	errorCount: number;
	totalBytes: number;
	bytesUploaded: number;
	// Groups render as one row, so they keep a single reason for the whole folder.
	lastError?: string;
}

// Non-reactive per-file bookkeeping. The aggregate fields above are kept in sync
// by applying byte deltas against these records, so a group's bytesUploaded is
// always the running sum of its files' bytes without ever reducing over them.
interface FileRec {
	name: string;
	size: number;
	bytes: number;
	status: "pending" | "uploading" | "complete" | "error";
	group?: string;
}

class UploadState {
	looseItems = $state<LooseItem[]>([]);
	groups = $state<UploadGroup[]>([]);
	minimized = $state(false);
	speed = $state(0);
	eta = $state<number | null>(null);
	// True while a drop/selection is being enumerated and enqueued. Serializes
	// the prepare→enqueue pipeline so overlapping drops can't race addFiles.
	preparing = $state(false);
	// True during UploadZone's (slow, silent) folder-tree enumeration, before
	// files are enqueued. Together with `preparing` it drives the panel's
	// "Preparing…" row, re-drop rejection, and the beforeunload guard.
	scanning = $state(false);
	// Monotonic drop token. UploadZone captures the value per drop and re-checks
	// it after each await; clear() bumps it, so a cancelled drop cannot resume
	// and cannot blank out a newer drop's scanning state.
	scanGen = 0;

	// Incrementally-maintained totals — never reduced over the item set.
	totalBytes = $state(0);
	totalBytesUploaded = $state(0);
	activeCount = $state(0);

	private autoMinimizeTimer: ReturnType<typeof setTimeout> | null = null;
	// Source of truth for per-file byte/status accounting (non-reactive).
	private fileIndex = new Map<string, FileRec>();
	// group id → reactive group proxy. Rebuilt from `groups` after any structural
	// change so lookups during 24k progress events stay O(1) and always point at
	// the live proxy (reassigning a $state array can swap proxy identities).
	private groupsById = new Map<string, UploadGroup>();

	hasItems = $derived(this.looseItems.length > 0 || this.groups.length > 0);
	// Never "complete" while a drop is still being enumerated/enqueued — an early
	// file can finish before later ones are added, which would otherwise flash
	// "complete" and auto-minimize the panel mid-drop.
	isComplete = $derived(
		this.hasItems && this.activeCount === 0 && !this.preparing && !this.scanning,
	);
	totalProgress = $derived(
		this.totalBytes === 0 ? 0 : Math.round((this.totalBytesUploaded / this.totalBytes) * 100),
	);

	private reindexGroups() {
		this.groupsById.clear();
		for (const g of this.groups) this.groupsById.set(g.id, g);
	}

	getGroup(groupId: string): UploadGroup | undefined {
		return this.groupsById.get(groupId);
	}

	// Single decrement path so lastError never outlives the failures it describes
	// and a re-failure cannot resurface the previous reason.
	private decErrorCount(g: UploadGroup) {
		g.errorCount--;
		if (g.errorCount <= 0) g.lastError = undefined;
	}

	addGroup(groupId: string, name: string, targetDir: string) {
		this.groups.push({
			id: groupId,
			name,
			targetDir,
			fileCount: 0,
			completedCount: 0,
			errorCount: 0,
			totalBytes: 0,
			bytesUploaded: 0,
			lastError: undefined,
		});
		this.reindexGroups();
	}

	// Register a group's full size up front — all files are known at enqueue time
	// even though they're fed to Uppy a window at a time, so the panel can show
	// the complete count immediately.
	setGroupTotals(groupId: string, fileCount: number, totalBytes: number) {
		const g = this.groupsById.get(groupId);
		if (!g) return;
		g.fileCount += fileCount;
		g.totalBytes += totalBytes;
		this.totalBytes += totalBytes;
		this.activeCount += fileCount;
		this.minimized = false;
		this.clearAutoMinimize();
	}

	// Begin per-file byte/status accounting for a windowed group file once Uppy
	// has assigned it an id. Group totals were already counted in setGroupTotals,
	// so this must NOT touch them. Returns whether the file is bound to `group`:
	// Uppy ids are deterministic, so a recovery re-drop collides with records
	// from a previous run. Terminal (complete/error) leftovers are released from
	// their old group and re-bound fresh; a record still actively uploading
	// under another group stays put, and the caller shrinks the new group by
	// the shortfall (adjustGroupShortfall).
	trackFile(id: string, size: number, group: string): boolean {
		const fi = this.fileIndex.get(id);
		if (!fi) {
			this.fileIndex.set(id, { name: "", size, bytes: 0, status: "pending", group });
			return true;
		}
		if (fi.status === "pending" || fi.status === "uploading") return fi.group === group;
		this.releaseRecord(id, fi);
		this.fileIndex.set(id, { name: fi.name, size, bytes: 0, status: "pending", group });
		return true;
	}

	// Detach a terminal (complete/error) record and every aggregate it holds a
	// stake in, so its deterministic id can be re-tracked by a new drop without
	// wedging the old group's counters.
	private releaseRecord(id: string, fi: FileRec) {
		this.totalBytes -= fi.size;
		this.totalBytesUploaded -= fi.bytes;
		if (fi.group) {
			const g = this.groupsById.get(fi.group);
			if (g) {
				g.fileCount--;
				g.totalBytes -= fi.size;
				g.bytesUploaded -= fi.bytes;
				if (fi.status === "complete") g.completedCount--;
				else this.decErrorCount(g);
				if (g.fileCount <= 0) {
					this.groups = this.groups.filter((x) => x.id !== g.id);
					this.reindexGroups();
				}
			}
		} else {
			this.looseItems = this.looseItems.filter((i) => i.id !== id);
		}
	}

	// A group's totals are registered up front from the raw drop list, but Uppy
	// can silently reject adds (duplicate ids) and a colliding record can stay
	// owned by another live group. Shrink this group by the shortfall so its
	// counters can still reach complete.
	adjustGroupShortfall(groupId: string, count: number, bytes: number) {
		const g = this.groupsById.get(groupId);
		if (!g) return;
		g.fileCount -= count;
		g.totalBytes -= bytes;
		this.totalBytes -= bytes;
		this.activeCount -= count;
		if (g.fileCount <= 0) {
			this.groups = this.groups.filter((x) => x.id !== g.id);
			this.reindexGroups();
			if (!this.hasItems) this.minimized = false;
		}
	}

	addLooseFiles(files: { id: string; name: string; size: number }[]) {
		let added = 0;
		let bytes = 0;
		for (const f of files) {
			if (this.fileIndex.has(f.id)) continue;
			this.fileIndex.set(f.id, { name: f.name, size: f.size, bytes: 0, status: "pending" });
			this.looseItems.push({
				id: f.id,
				name: f.name,
				size: f.size,
				bytesUploaded: 0,
				progress: 0,
				status: "pending",
			});
			added++;
			bytes += f.size;
		}
		this.totalBytes += bytes;
		this.activeCount += added;
		this.minimized = false;
		this.clearAutoMinimize();
	}

	private looseItem(id: string): LooseItem | undefined {
		// looseItems is small (loose drops are tens of files), so a scan is cheap.
		return this.looseItems.find((i) => i.id === id);
	}

	updateProgress(id: string, bytesUploaded: number) {
		const fi = this.fileIndex.get(id);
		if (!fi || fi.status === "complete") return;
		const delta = bytesUploaded - fi.bytes;
		fi.bytes = bytesUploaded;
		if (fi.status === "pending") {
			fi.status = "uploading";
		} else if (fi.status === "error") {
			// Uppy's upload() retries every errored file instance-wide, so a failed
			// file can resume without an explicit retry() call. Re-arm the counters
			// exactly like retry() — otherwise markComplete double-decrements
			// activeCount and errorCount goes stale.
			fi.status = "uploading";
			this.activeCount++;
			if (fi.group) {
				const g = this.groupsById.get(fi.group);
				if (g) this.decErrorCount(g);
			}
		}
		this.totalBytesUploaded += delta;
		if (fi.group) {
			const g = this.groupsById.get(fi.group);
			if (g) g.bytesUploaded += delta;
		} else {
			const item = this.looseItem(id);
			if (item) {
				item.bytesUploaded = bytesUploaded;
				item.progress = item.size > 0 ? Math.round((bytesUploaded / item.size) * 100) : 0;
				item.status = "uploading";
				item.error = undefined;
			}
		}
	}

	updateSpeedAndEta(speed: number, eta: number | null) {
		this.speed = speed;
		this.eta = eta;
	}

	markComplete(id: string) {
		const fi = this.fileIndex.get(id);
		if (!fi || fi.status === "complete") return;
		const wasActive = fi.status === "pending" || fi.status === "uploading";
		// A retried file can complete while still marked error (no progress event
		// fired) — move it out of the error tally so counts stay exact.
		const wasError = fi.status === "error";
		const delta = fi.size - fi.bytes;
		fi.bytes = fi.size;
		fi.status = "complete";
		this.totalBytesUploaded += delta;
		if (wasActive) this.activeCount--;
		if (fi.group) {
			const g = this.groupsById.get(fi.group);
			if (g) {
				g.completedCount++;
				if (wasError) this.decErrorCount(g);
				g.bytesUploaded += delta;
			}
		} else {
			const item = this.looseItem(id);
			if (item) {
				item.bytesUploaded = item.size;
				item.progress = 100;
				item.status = "complete";
				item.error = undefined;
			}
		}
		this.checkAutoMinimize();
	}

	markError(id: string, error: string) {
		const fi = this.fileIndex.get(id);
		if (!fi || fi.status === "error") return;
		const wasActive = fi.status === "pending" || fi.status === "uploading";
		fi.status = "error";
		if (wasActive) this.activeCount--;
		if (fi.group) {
			const g = this.groupsById.get(fi.group);
			if (g) {
				g.errorCount++;
				// First failure wins: one blocked parent fails every file under it.
				g.lastError ??= error;
			}
		} else {
			const item = this.looseItem(id);
			if (item) {
				item.status = "error";
				item.error = error;
			}
		}
	}

	// Re-arm a failed file for retry: back to active, error cleared, bytes reset
	// so the resumed progress recomputes cleanly.
	retry(id: string) {
		const fi = this.fileIndex.get(id);
		if (!fi || fi.status !== "error") return;
		this.totalBytesUploaded -= fi.bytes;
		if (fi.group) {
			const g = this.groupsById.get(fi.group);
			if (g) {
				g.bytesUploaded -= fi.bytes;
				this.decErrorCount(g);
			}
		} else {
			const item = this.looseItem(id);
			if (item) {
				item.status = "pending";
				item.error = undefined;
				item.bytesUploaded = 0;
				item.progress = 0;
			}
		}
		fi.bytes = 0;
		fi.status = "pending";
		this.activeCount++;
	}

	// Cancel a single loose file (the per-file X). Groups cancel via removeGroup.
	removeFile(id: string) {
		const fi = this.fileIndex.get(id);
		if (!fi) return;
		const wasActive = fi.status === "pending" || fi.status === "uploading";
		this.totalBytes -= fi.size;
		this.totalBytesUploaded -= fi.bytes;
		if (wasActive) this.activeCount--;
		this.fileIndex.delete(id);
		if (fi.group) {
			const g = this.groupsById.get(fi.group);
			if (g) {
				g.fileCount--;
				g.totalBytes -= fi.size;
				g.bytesUploaded -= fi.bytes;
				if (fi.status === "complete") g.completedCount--;
				else if (fi.status === "error") this.decErrorCount(g);
				if (g.fileCount <= 0) {
					this.groups = this.groups.filter((x) => x.id !== g.id);
					this.reindexGroups();
				}
			}
		} else {
			this.looseItems = this.looseItems.filter((i) => i.id !== id);
		}
		if (!this.hasItems) this.minimized = false;
	}

	// Whether a file belongs to a folder group — grouped files have no per-file
	// retry UI, so error handling treats them differently from loose files.
	isGrouped(id: string): boolean {
		return !!this.fileIndex.get(id)?.group;
	}

	groupFileIds(groupId: string): string[] {
		const ids: string[] = [];
		for (const [id, fi] of this.fileIndex) if (fi.group === groupId) ids.push(id);
		return ids;
	}

	removeGroup(groupId: string) {
		const g = this.groupsById.get(groupId);
		if (!g) return;
		this.totalBytes -= g.totalBytes;
		this.totalBytesUploaded -= g.bytesUploaded;
		this.activeCount -= g.fileCount - g.completedCount - g.errorCount;
		for (const [id, fi] of this.fileIndex) if (fi.group === groupId) this.fileIndex.delete(id);
		this.groups = this.groups.filter((x) => x.id !== groupId);
		this.reindexGroups();
		if (!this.hasItems) this.minimized = false;
	}

	clearCompleted() {
		// Drop completed loose rows.
		const keptLoose: LooseItem[] = [];
		for (const i of this.looseItems) {
			if (i.status === "complete") {
				const fi = this.fileIndex.get(i.id);
				if (fi) {
					this.totalBytes -= fi.size;
					this.totalBytesUploaded -= fi.bytes;
					this.fileIndex.delete(i.id);
				}
			} else {
				keptLoose.push(i);
			}
		}
		this.looseItems = keptLoose;

		// Drop fully-complete groups (keep any with errors so they stay visible).
		const keptGroups: UploadGroup[] = [];
		for (const g of this.groups) {
			const fullyComplete = g.fileCount > 0 && g.errorCount === 0 && g.completedCount >= g.fileCount;
			if (fullyComplete) {
				this.totalBytes -= g.totalBytes;
				this.totalBytesUploaded -= g.bytesUploaded;
				for (const [id, fi] of this.fileIndex) if (fi.group === g.id) this.fileIndex.delete(id);
			} else {
				keptGroups.push(g);
			}
		}
		this.groups = keptGroups;
		this.reindexGroups();
		if (!this.hasItems) this.minimized = false;
	}

	// Mark any still-active file Uppy no longer tracks as complete — a safety net
	// for the rare case a file leaves Uppy's queue without a success/error event.
	reconcileActive(trackedIds: Set<string>) {
		for (const [id, fi] of this.fileIndex) {
			if ((fi.status === "pending" || fi.status === "uploading") && !trackedIds.has(id)) {
				this.markComplete(id);
			}
		}
	}

	clear() {
		this.looseItems = [];
		this.groups = [];
		this.fileIndex.clear();
		this.groupsById.clear();
		this.totalBytes = 0;
		this.totalBytesUploaded = 0;
		this.activeCount = 0;
		this.speed = 0;
		this.eta = null;
		this.minimized = false;
		// Cancelling mid-scan also aborts any in-progress folder enumeration:
		// bumping scanGen invalidates the generation UploadZone captured at drop
		// time; the boolean only resets the UI display.
		this.scanGen++;
		this.scanning = false;
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
