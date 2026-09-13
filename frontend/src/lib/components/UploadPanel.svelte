<script lang="ts">
	import { uploadState } from "$lib/stores/upload.svelte.js";
	import { audioPlayer } from "$lib/stores/audioPlayer.svelte.js";
	import { cancelUpload, cancelGroup, cancelAll, retryUpload } from "$lib/upload/uppy.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import XIcon from "@lucide/svelte/icons/x";
	import ChevronUpIcon from "@lucide/svelte/icons/chevron-up";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import RotateCwIcon from "@lucide/svelte/icons/rotate-cw";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import CancelUploadDialog from "$lib/components/dialogs/CancelUploadDialog.svelte";

	interface DisplayEntry {
		type: "file" | "directory";
		id: string;
		name: string;
		size: number;
		bytesUploaded: number;
		progress: number;
		status: "pending" | "uploading" | "complete" | "error";
		fileCount?: number;
		completedCount?: number;
		error?: string;
	}

	const displayItems = $derived.by((): DisplayEntry[] => {
		const entries: DisplayEntry[] = [];
		for (const g of uploadState.groups) {
			const finished = g.completedCount + g.errorCount;
			const status =
				finished >= g.fileCount
					? g.errorCount > 0
						? "error"
						: "complete"
					: g.bytesUploaded > 0
						? "uploading"
						: "pending";
			entries.push({
				type: "directory",
				id: g.id,
				name: g.name,
				size: g.totalBytes,
				bytesUploaded: g.bytesUploaded,
				progress: g.totalBytes > 0 ? Math.round((g.bytesUploaded / g.totalBytes) * 100) : 0,
				status,
				fileCount: g.fileCount,
				completedCount: g.completedCount,
				error: g.lastError,
			});
		}
		for (const item of uploadState.looseItems) {
			entries.push({
				type: "file",
				id: item.id,
				name: item.name,
				size: item.size,
				bytesUploaded: item.bytesUploaded,
				progress: item.progress,
				status: item.status,
				error: item.error,
			});
		}
		return entries;
	});

	const DETAIL_THRESHOLD = 20;

	// File-level totals for the header — a group contributes all its files, not
	// the single row it renders as.
	const totalFileCount = $derived(
		uploadState.looseItems.length + uploadState.groups.reduce((s, g) => s + g.fileCount, 0),
	);
	const totalCompleteCount = $derived(
		uploadState.looseItems.filter((i) => i.status === "complete").length +
			uploadState.groups.reduce((s, g) => s + g.completedCount, 0),
	);
	const totalErrorCount = $derived(
		uploadState.looseItems.filter((i) => i.status === "error").length +
			uploadState.groups.reduce((s, g) => s + g.errorCount, 0),
	);

	const errorEntries = $derived(displayItems.filter((i) => i.status === "error"));
	const isLargeBatch = $derived(displayItems.length > DETAIL_THRESHOLD);
	const isStalled = $derived(
		uploadState.activeCount > 0 && uploadState.speed < 1024 && uploadState.totalProgress > 0,
	);

	// While uploading with throughput but no ETA yet, the rate is still warming up
	// (see ETA_WARMUP_SAMPLES in uppy.ts) — show "estimating…" instead of a number.
	const etaText = $derived.by(() => {
		if (uploadState.eta !== null) return formatEta(uploadState.eta);
		if (uploadState.activeCount > 0 && uploadState.speed > 0) return "estimating…";
		return "";
	});

	const singleGroupName = $derived(
		uploadState.groups.length === 1 && uploadState.looseItems.length === 0
			? uploadState.groups[0].name
			: null,
	);

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
	}

	function formatSpeed(bytesPerSec: number): string {
		if (bytesPerSec < 1024) return `${Math.round(bytesPerSec)} B/s`;
		if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`;
		if (bytesPerSec < 1024 * 1024 * 1024) return `${(bytesPerSec / (1024 * 1024)).toFixed(1)} MB/s`;
		return `${(bytesPerSec / (1024 * 1024 * 1024)).toFixed(1)} GB/s`;
	}

	// Coarse buckets, not exact seconds — the underlying ETA twitches with every
	// burst of small-file completions, so rounding to friendly units (like Windows)
	// keeps the displayed number from dancing.
	function formatEta(seconds: number | null): string {
		if (seconds === null || seconds <= 0) return "";
		if (seconds > 86400) return "calculating...";
		if (seconds < 60) {
			const s = Math.max(5, Math.round(seconds / 5) * 5);
			return `${s}s left`;
		}
		if (seconds < 3600) {
			const m = Math.max(1, Math.round(seconds / 60));
			return `${m}m left`;
		}
		const h = Math.floor(seconds / 3600);
		const m = Math.round((seconds % 3600) / 60);
		return m > 0 ? `${h}h ${m}m left` : `${h}h left`;
	}

	function handleCancel(entry: DisplayEntry) {
		if (entry.type === "directory") {
			cancelGroup(entry.id);
		} else {
			cancelUpload(entry.id);
		}
	}

	// Cancelling a long upload is a one-click way to throw away a lot of work, so
	// gate the cancel-all paths behind an inline confirm.
	let confirmingCancel = $state(false);

	function requestCancelAll() {
		confirmingCancel = true;
	}

	function confirmCancelAll() {
		confirmingCancel = false;
		cancelAll();
	}

	$effect(() => {
		if (!uploadState.hasItems && !uploadState.preparing && !uploadState.scanning) {
			confirmingCancel = false;
		}
	});
</script>

{#if uploadState.hasItems || uploadState.preparing || uploadState.scanning}
	<div
		class="fixed right-4 z-40 w-96 max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-border-2 bg-card {audioPlayer.visible ? 'bottom-[168px] md:bottom-[100px]' : 'bottom-[84px] md:bottom-4'}"
	>
		<!-- Preparing: enumeration/enqueue phase, before items exist -->
		{#if uploadState.preparing || uploadState.scanning}
			<div class="flex items-center gap-2.5 px-3 py-2.5 {uploadState.hasItems ? 'border-b border-border' : ''}">
				<div class="size-4 animate-spin rounded-full border-2 border-muted-foreground/40 border-t-accent-brand"></div>
				<span class="flex-1 text-sm font-medium">{uploadState.hasItems ? "Adding files…" : "Preparing upload…"}</span>
				<Button
					variant="destructive"
					size="icon-sm"
					onclick={() => requestCancelAll()}
					title="Cancel"
					aria-label="Cancel upload"
				>
					<XIcon class="size-4" />
				</Button>
			</div>
		{/if}

		{#if uploadState.hasItems}
		<!-- Header -->
		<button
			class="flex w-full items-center justify-between px-3 py-2.5 hover:bg-muted"
			onclick={() => (uploadState.minimized = !uploadState.minimized)}
		>
			<span class="text-sm font-medium">
				{#if uploadState.isComplete}
					{#if totalErrorCount > 0 && totalCompleteCount === 0}
						{totalErrorCount} upload{totalErrorCount !== 1 ? "s" : ""} failed
					{:else if totalErrorCount > 0}
						{totalCompleteCount} complete · {totalErrorCount} failed
					{:else}
						{totalFileCount} upload{totalFileCount !== 1 ? "s" : ""} complete
					{/if}
				{:else}
					{#if uploadState.totalProgress >= 100}
						Finalizing...
					{:else}
						{#if singleGroupName}
							Uploading {singleGroupName}/
						{:else}
							Uploading {uploadState.activeCount} file{uploadState.activeCount !== 1 ? "s" : ""}...
						{/if}
						{uploadState.totalProgress}%
						{#if isStalled}
							<span class="text-muted-foreground"> · Stalled</span>
						{:else if uploadState.speed > 0}
							<span class="text-muted-foreground">
								· {formatSpeed(uploadState.speed)}
								{#if etaText}
									· {etaText}
								{/if}
							</span>
						{/if}
					{/if}
				{/if}
			</span>
			<div class="flex items-center gap-1">
				{#if uploadState.isComplete}
					<Button
						variant="ghost"
						size="icon-xs"
						onclick={(e) => { e.stopPropagation(); uploadState.clear(); }}
						title="Dismiss"
						aria-label="Dismiss upload panel"
					>
						<XIcon class="size-3.5" />
					</Button>
				{:else}
					<Button
						variant="destructive"
						size="icon-sm"
						onclick={(e) => { e.stopPropagation(); requestCancelAll(); }}
						title="Cancel all"
						aria-label="Cancel all uploads"
					>
						<XIcon class="size-4" />
					</Button>
				{/if}
				{#if uploadState.minimized}
					<ChevronUpIcon class="size-4 text-muted-foreground" />
				{:else}
					<ChevronDownIcon class="size-4 text-muted-foreground" />
				{/if}
			</div>
		</button>

		<!-- Progress bar (always visible) -->
		{#if !uploadState.isComplete}
			<div class="h-0.5 bg-muted">
				<div
					class="h-full bg-accent-brand transition-all"
					style="width: {uploadState.totalProgress}%"
				></div>
			</div>
		{/if}

		<!-- File list -->
		{#if !uploadState.minimized}
			{#if isLargeBatch}
				<!-- Summary view for large batches -->
				<div class="border-t border-border px-3 py-2">
					<div class="text-xs text-muted-foreground">
						{totalCompleteCount} of {totalFileCount} items complete
					</div>
					{#if uploadState.activeCount > 0 && uploadState.speed > 0}
						<div class="font-mono text-[11px] text-muted-foreground">
							{formatSize(uploadState.totalBytesUploaded)} / {formatSize(uploadState.totalBytes)}
							· {formatSpeed(uploadState.speed)}
							{#if etaText}
								· {etaText}
							{/if}
						</div>
					{/if}
				</div>
				{#if errorEntries.length > 0}
					<div class="max-h-40 overflow-y-auto">
						{#each errorEntries as entry (entry.id)}
							<div class="flex items-center gap-2 border-t border-border px-3 py-1.5">
								<div class="shrink-0">
									<AlertCircleIcon class="size-3.5 text-destructive" />
								</div>
								<div class="min-w-0 flex-1">
									<div class="truncate text-xs">{entry.name}</div>
									<span class="truncate text-[10px] text-destructive">{entry.error ?? "Upload failed"}</span>
								</div>
								<div class="shrink-0">
									{#if entry.type === "file"}
										<Button
											variant="ghost"
											size="icon-xs"
											onclick={() => retryUpload(entry.id)}
											title="Retry"
											aria-label="Retry upload of {entry.name}"
										>
											<RotateCwIcon class="size-3" />
										</Button>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			{:else}
				<div class="max-h-64 overflow-y-auto">
					{#each displayItems as entry (entry.id)}
						<div class="flex items-center gap-2 border-t border-border px-3 py-1.5">
							<!-- Status icon -->
							<div class="shrink-0">
								{#if entry.status === "complete"}
									<CheckIcon class="size-3.5 text-accent-brand" />
								{:else if entry.status === "error"}
									<AlertCircleIcon class="size-3.5 text-destructive" />
								{:else if entry.type === "directory"}
									<FolderIcon class="size-3.5 text-muted-foreground" />
								{:else}
									<div class="size-3.5 animate-spin rounded-full border-2 border-muted-foreground border-t-accent-brand"></div>
								{/if}
							</div>

							<!-- File/directory info -->
							<div class="min-w-0 flex-1">
								<div class="truncate text-xs">
									{entry.name}{entry.type === "directory" ? "/" : ""}
								</div>
								<div class="flex items-center gap-2">
									{#if entry.status === "error"}
										<span class="truncate font-mono text-[11px] text-destructive">{entry.error ?? "Upload failed"}</span>
									{:else if entry.status === "complete"}
										<span class="font-mono text-[11px] text-muted-foreground">
											{#if entry.type === "directory"}
												{entry.fileCount} file{entry.fileCount !== 1 ? "s" : ""} · {formatSize(entry.size)}
											{:else}
												{formatSize(entry.size)}
											{/if}
										</span>
									{:else}
										<div class="h-1 flex-1 rounded-full bg-muted">
											<div
												class="h-full rounded-full bg-accent-brand transition-all"
												style="width: {entry.progress}%"
											></div>
										</div>
										<span class="font-mono text-[11px] text-muted-foreground">
											{#if entry.type === "directory"}
												{entry.completedCount}/{entry.fileCount} · {formatSize(entry.bytesUploaded)} / {formatSize(entry.size)}
											{:else}
												{formatSize(entry.bytesUploaded)} / {formatSize(entry.size)}
											{/if}
										</span>
									{/if}
								</div>
							</div>

							<!-- Actions -->
							<div class="shrink-0">
								{#if entry.status === "error" && entry.type === "file"}
									<Button
										variant="ghost"
										size="icon-xs"
										onclick={() => retryUpload(entry.id)}
										title="Retry"
										aria-label="Retry upload of {entry.name}"
									>
										<RotateCwIcon class="size-3" />
									</Button>
								{:else if entry.status !== "complete" && displayItems.length > 1}
									<!-- With a single entry the top-right X already cancels it; a
									     per-row X would be redundant. -->
									<Button
										variant="ghost"
										size="icon-xs"
										onclick={() => handleCancel(entry)}
										title="Cancel"
										aria-label="Cancel upload of {entry.name}"
									>
										<XIcon class="size-3" />
									</Button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}

			{#if uploadState.isComplete}
				<div class="border-t border-border px-3 py-1.5">
					<Button
						variant="ghost"
						size="xs"
						class="w-full"
						onclick={() => uploadState.clear()}
					>
						Dismiss
					</Button>
				</div>
			{/if}
		{/if}
		{/if}
	</div>
{/if}

{#if confirmingCancel && !uploadState.isComplete}
	<CancelUploadDialog
		onconfirm={() => confirmCancelAll()}
		oncancel={() => (confirmingCancel = false)}
	/>
{/if}
