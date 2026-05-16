<script lang="ts">
	import { uploadState } from "$lib/stores/upload.svelte.js";
	import { cancelUpload, retryUpload } from "$lib/upload/uppy.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import XIcon from "@lucide/svelte/icons/x";
	import ChevronUpIcon from "@lucide/svelte/icons/chevron-up";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import RotateCwIcon from "@lucide/svelte/icons/rotate-cw";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";

	const DETAIL_THRESHOLD = 20;

	const errorItems = $derived(uploadState.items.filter((i) => i.status === "error"));
	const completedCount = $derived(uploadState.items.filter((i) => i.status === "complete").length);
	const isLargeBatch = $derived(uploadState.items.length > DETAIL_THRESHOLD);
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
						{completedCount} of {uploadState.items.length} files complete
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
				{#if errorItems.length > 0}
					<div class="max-h-40 overflow-y-auto">
						{#each errorItems as item (item.id)}
							<div class="flex items-center gap-2 border-t border-border px-3 py-1.5">
								<div class="shrink-0">
									<AlertCircleIcon class="size-3.5 text-destructive" />
								</div>
								<div class="min-w-0 flex-1">
									<div class="truncate text-xs">{item.name}</div>
									<span class="truncate text-[10px] text-destructive">{item.error}</span>
								</div>
								<div class="shrink-0">
									<Button
										variant="ghost"
										size="icon-xs"
										onclick={() => retryUpload(item.id)}
										title="Retry"
									>
										<RotateCwIcon class="size-3" />
									</Button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			{:else}
				<div class="max-h-64 overflow-y-auto">
					{#each uploadState.items as item (item.id)}
						<div class="flex items-center gap-2 border-t border-border px-3 py-1.5">
							<!-- Status icon -->
							<div class="shrink-0">
								{#if item.status === "complete"}
									<CheckIcon class="size-3.5 text-green-500" />
								{:else if item.status === "error"}
									<AlertCircleIcon class="size-3.5 text-destructive" />
								{:else}
									<div class="size-3.5 animate-spin rounded-full border-2 border-muted-foreground border-t-primary"></div>
								{/if}
							</div>

							<!-- File info -->
							<div class="min-w-0 flex-1">
								<div class="truncate text-xs">{item.name}</div>
								<div class="flex items-center gap-2">
									{#if item.status === "error"}
										<span class="truncate text-[10px] text-destructive">{item.error}</span>
									{:else if item.status === "complete"}
										<span class="text-[10px] text-muted-foreground">{formatSize(item.size)}</span>
									{:else}
										<div class="h-1 flex-1 rounded-full bg-muted">
											<div
												class="h-full rounded-full bg-primary transition-all"
												style="width: {item.progress}%"
											></div>
										</div>
										<span class="text-[10px] text-muted-foreground">
										{formatSize(item.bytesUploaded)} / {formatSize(item.size)}
									</span>
									{/if}
								</div>
							</div>

							<!-- Actions -->
							<div class="shrink-0">
								{#if item.status === "error"}
									<Button
										variant="ghost"
										size="icon-xs"
										onclick={() => retryUpload(item.id)}
										title="Retry"
									>
										<RotateCwIcon class="size-3" />
									</Button>
								{:else if item.status !== "complete"}
									<Button
										variant="ghost"
										size="icon-xs"
										onclick={() => cancelUpload(item.id)}
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
