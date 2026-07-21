<script lang="ts">
	interface Props {
		href?: string;
	}
	let { href = "/files" }: Props = $props();
	const gradientId = `onyx-bm-${Math.random().toString(36).slice(2, 10)}`;
</script>

<a
	{href}
	class="group flex shrink-0 items-center gap-[10px] rounded-md outline-offset-4"
	aria-label="Onyx home"
>
	<svg
		width="24"
		height="24"
		viewBox="0 0 24 24"
		aria-hidden="true"
		class="block shrink-0"
	>
		<defs>
			<linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
				<stop offset="0%" stop-color="oklch(0.82 0.12 245)" />
				<stop offset="100%" stop-color="oklch(0.55 0.13 245)" />
			</linearGradient>
		</defs>
		<path d="M12 2 L22 12 L12 22 L2 12 Z" fill="url(#{gradientId})" />
		<path d="M12 2 L22 12 L12 12 Z" fill="oklch(1 0 0 / 0.2)" />
		<path d="M2 12 L12 12 L12 22 Z" fill="oklch(0 0 0 / 0.24)" />
	</svg>
	<span class="text-lg font-bold tracking-[-0.01em] text-foreground max-md:text-base">Onyx</span>
</a>
