<script lang="ts">
	import { uploadState } from "$lib/stores/upload.svelte.js";
	import { cancelUpload, cancelGroup, retryUpload } from "$lib/upload/uppy.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import XIcon from "@lucide/svelte/icons/x";
	import ChevronUpIcon from "@lucide/svelte/icons/chevron-up";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import RotateCwIcon from "@lucide/svelte/icons/rotate-cw";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import FolderIcon from "@lucide/svelte/icons/folder";

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
		const seenGroups = new Set<string>();

		for (const item of uploadState.items) {
			if (item.group) {
				if (seenGroups.has(item.group)) continue;
				seenGroups.add(item.group);

				const groupItems = uploadState.items.filter((i) => i.group === item.group);
				const meta = uploadState.groupMeta[item.group];
				const totalSize = groupItems.reduce((s, i) => s + i.size, 0);
				const totalUploaded = groupItems.reduce((s, i) => s + i.bytesUploaded, 0);
				const completed = groupItems.filter((i) => i.status === "complete").length;
				const allComplete = completed === groupItems.length;
				const hasError = groupItems.some((i) => i.status === "error");
				const hasUploading = groupItems.some((i) => i.status === "uploading");

				entries.push({
					type: "directory",
					id: item.group,
					name: meta?.name ?? "Directory",
					size: totalSize,
					bytesUploaded: totalUploaded,
					progress: totalSize > 0 ? Math.round((totalUploaded / totalSize) * 100) : 0,
					status: allComplete ? "complete" : hasError ? "error" : hasUploading ? "uploading" : "pending",
					fileCount: groupItems.length,
					completedCount: completed,
				});
			} else {
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
		}

		return entries;
	});

	const DETAIL_THRESHOLD = 20;

	const errorEntries = $derived(displayItems.filter((i) => i.status === "error"));
	const completedCount = $derived(displayItems.filter((i) => i.status === "complete").length);
	const isLargeBatch = $derived(displayItems.length > DETAIL_THRESHOLD);
	const isStalled = $derived(
		uploadState.activeCount > 0 && uploadState.speed < 1024 && uploadState.totalProgress > 0,
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

	function formatEta(seconds: number | null): string {
		if (seconds === null || seconds <= 0) return "";
		if (seconds > 86400) return "calculating...";
		if (seconds < 60) return `${Math.ceil(seconds)}s left`;
		if (seconds < 3600) {
			const m = Math.floor(seconds / 60);
			const s = Math.ceil(seconds % 60);
			return `${m}m ${s}s left`;
		}
		const h = Math.floor(seconds / 3600);
		const m = Math.ceil((seconds % 3600) / 60);
		return `${h}h ${m}m left`;
	}

	function handleCancel(entry: DisplayEntry) {
		if (entry.type === "directory") {
			cancelGroup(entry.id);
		} else {
			cancelUpload(entry.id);
		}
	}
</script>

{#if uploadState.hasItems}
	<div class="fixed bottom-0 right-0 z-40 w-96 rounded-tl-lg border border-border bg-background shadow-lg">
		<!-- Header -->
		<button
			class="flex w-full items-center justify-between px-3 py-2 hover:bg-muted/50"
			onclick={() => (uploadState.minimized = !uploadState.minimized)}
		>
			<span class="text-sm font-medium">
				{#if uploadState.isComplete}
					{uploadState.items.length} upload{uploadState.items.length !== 1 ? "s" : ""} complete
				{:else}
					Uploading {uploadState.activeCount} file{uploadState.activeCount !== 1 ? "s" : ""}...
					{uploadState.totalProgress}%
					{#if isStalled}
						<span class="text-muted-foreground"> · Stalled</span>
					{:else if uploadState.speed > 0}
						<span class="text-muted-foreground">
							· {formatSpeed(uploadState.speed)}
							{#if uploadState.eta !== null}
								· {formatEta(uploadState.eta)}
							{/if}
						</span>
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
					>
						<XIcon class="size-3.5" />
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
					class="h-full bg-primary transition-all"
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
						{completedCount} of {displayItems.length} items complete
					</div>
					{#if uploadState.activeCount > 0 && uploadState.speed > 0}
						<div class="text-[10px] text-muted-foreground">
							{formatSize(uploadState.totalBytesUploaded)} / {formatSize(uploadState.totalBytes)}
							· {formatSpeed(uploadState.speed)}
							{#if uploadState.eta !== null}
								· {formatEta(uploadState.eta)}
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
									<CheckIcon class="size-3.5 text-green-500" />
								{:else if entry.status === "error"}
									<AlertCircleIcon class="size-3.5 text-destructive" />
								{:else if entry.type === "directory"}
									<FolderIcon class="size-3.5 text-muted-foreground" />
								{:else}
									<div class="size-3.5 animate-spin rounded-full border-2 border-muted-foreground border-t-primary"></div>
								{/if}
							</div>

							<!-- File/directory info -->
							<div class="min-w-0 flex-1">
								<div class="truncate text-xs">
									{entry.name}{entry.type === "directory" ? "/" : ""}
								</div>
								<div class="flex items-center gap-2">
									{#if entry.status === "error"}
										<span class="truncate text-[10px] text-destructive">{entry.error ?? "Upload failed"}</span>
									{:else if entry.status === "complete"}
										<span class="text-[10px] text-muted-foreground">
											{#if entry.type === "directory"}
												{entry.fileCount} file{entry.fileCount !== 1 ? "s" : ""} · {formatSize(entry.size)}
											{:else}
												{formatSize(entry.size)}
											{/if}
										</span>
									{:else}
										<div class="h-1 flex-1 rounded-full bg-muted">
											<div
												class="h-full rounded-full bg-primary transition-all"
												style="width: {entry.progress}%"
											></div>
										</div>
										<span class="text-[10px] text-muted-foreground">
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
									>
										<RotateCwIcon class="size-3" />
									</Button>
								{:else if entry.status !== "complete"}
									<Button
										variant="ghost"
										size="icon-xs"
										onclick={() => handleCancel(entry)}
										title="Cancel"
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
	</div>
{/if}
