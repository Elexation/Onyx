<script lang="ts">
	let { token }: { token: string } = $props();
</script>

<header class="flex h-14 items-center gap-3 border-b border-border px-5">
	<svg
		width="22"
		height="22"
		viewBox="0 0 24 24"
		aria-hidden="true"
		class="shrink-0"
	>
		<defs>
			<linearGradient id="onyx-share-mark" x1="0" y1="0" x2="1" y2="1">
				<stop offset="0%" stop-color="oklch(0.82 0.12 245)" />
				<stop offset="100%" stop-color="oklch(0.55 0.13 245)" />
			</linearGradient>
		</defs>
		<path d="M12 2 L22 12 L12 22 L2 12 Z" fill="url(#onyx-share-mark)" />
		<path d="M12 2 L22 12 L12 12 Z" fill="oklch(1 0 0 / 0.2)" />
		<path d="M2 12 L12 12 L12 22 Z" fill="oklch(0 0 0 / 0.24)" />
	</svg>
	<span class="text-[15px] font-semibold tracking-[-0.01em]">Onyx</span>
	<span class="ml-auto font-mono text-[11px] text-muted-foreground">/s/{token}</span>
</header>
