<script lang="ts">
	import { selection } from "$lib/stores/selection.svelte.js";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { X, Copy, Download, Share2, Trash2 } from "lucide-svelte";

	let {
		oncopy,
		ondownload,
		onshare,
		ondelete,
	}: {
		oncopy: () => void;
		ondownload: () => void;
		onshare: () => void;
		ondelete: () => void;
	} = $props();

	const visible = $derived(selection.count > 0);
</script>

{#if visible}
	<div
		class="fixed inset-x-3 bottom-5 z-40 flex items-center gap-2 rounded-2xl border border-border-2 bg-popover px-3 py-2.5 md:hidden"
		role="toolbar"
		aria-label="Selection actions"
	>
		<button
			type="button"
			class="inline-flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
			aria-label="Clear selection"
			onclick={() => selection.clear()}
		>
			<X class="size-[18px]" strokeWidth={2} />
		</button>
		<span class="text-sm font-semibold text-foreground">{selection.count} selected</span>
		<div class="ml-auto flex items-center gap-0.5">
			<button
				type="button"
				class="inline-flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				aria-label="Copy"
				title="Copy"
				onclick={oncopy}
			>
				<Copy class="size-4" strokeWidth={2} />
			</button>
			<button
				type="button"
				class="inline-flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				aria-label="Download"
				title="Download"
				onclick={ondownload}
			>
				<Download class="size-4" strokeWidth={2} />
			</button>
			{#if sharesEnabled.enabled}
				<button
					type="button"
					class="inline-flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
					aria-label="Share"
					title="Share"
					onclick={onshare}
				>
					<Share2 class="size-4" strokeWidth={2} />
				</button>
			{/if}
			<button
				type="button"
				class="inline-flex size-9 items-center justify-center rounded-lg text-destructive transition-colors hover:bg-destructive/10"
				aria-label="Delete"
				title="Delete"
				onclick={ondelete}
			>
				<Trash2 class="size-4" strokeWidth={2} />
			</button>
		</div>
	</div>
{/if}
