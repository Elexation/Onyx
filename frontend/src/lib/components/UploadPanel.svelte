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

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
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
									<span class="text-[10px] text-muted-foreground">{item.progress}%</span>
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
