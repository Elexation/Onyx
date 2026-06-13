<script lang="ts">
	import { HardDrive } from "lucide-svelte";
	import { onMount } from "svelte";
	import { getStorageUsage } from "$lib/api/storage.js";
	import { formatFileSize } from "$lib/utils/format.js";

	let hostname = $state("—");
	let used = $state<number | null>(null);
	let total = $state<number | null>(null);
	let errored = $state(false);

	const pct = $derived(
		used !== null && total !== null && total > 0
			? Math.min(100, Math.max(0, (used / total) * 100))
			: 0,
	);

	async function load() {
		try {
			const data = await getStorageUsage();
			used = data.used;
			total = data.total;
			errored = false;
		} catch {
			used = null;
			total = null;
			errored = true;
		}
	}

	onMount(() => {
		if (typeof window !== "undefined") {
			hostname = window.location.hostname || "localhost";
		}
		load();
		const id = window.setInterval(load, 30_000);
		return () => window.clearInterval(id);
	});
</script>

<div class="mt-auto flex flex-col gap-1.5 border-t border-border pt-[14px]">
	<div class="flex items-center justify-between text-xs text-muted-foreground">
		<span class="inline-flex items-center gap-1.5">
			<HardDrive class="size-[13px]" strokeWidth={2} />
			Storage
		</span>
		<span class="font-mono">
			{#if used !== null && total !== null}
				{formatFileSize(used)} / {formatFileSize(total)}
			{:else}
				—
			{/if}
		</span>
	</div>
	<div class="h-1.5 overflow-hidden rounded-[3px] bg-muted" aria-hidden="true">
		<div
			class="h-full rounded-[3px] bg-accent-brand transition-[width] duration-300"
			style="width: {pct}%;"
		></div>
	</div>
	<div class="flex items-center justify-between font-mono text-[11px] text-muted-foreground">
		<span class="truncate" title={hostname}>{hostname}</span>
		<span class="shrink-0 {errored ? 'text-destructive' : 'text-accent-brand'}">
			● {errored ? "unreachable" : "healthy"}
		</span>
	</div>
</div>
