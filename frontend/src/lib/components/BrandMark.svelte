<script lang="ts">
	interface Props {
		href?: string;
	}
	let { href = "/files" }: Props = $props();
</script>

<a
	{href}
	class="group flex shrink-0 items-center gap-[10px] rounded-md outline-offset-4"
	aria-label="Onyx home"
>
	<span
		class="inline-flex size-7 items-center justify-center rounded-lg text-accent-brand-foreground"
		style="background: linear-gradient(135deg, var(--accent-brand) 0%, oklch(0.5 0.13 245) 100%);"
	>
		<svg
			width="16"
			height="16"
			viewBox="0 0 24 24"
			fill="currentColor"
			aria-hidden="true"
		>
			<path d="M12 2 2 7v10l10 5 10-5V7L12 2zm0 2.18L19.82 8 12 11.82 4.18 8 12 4.18z" />
		</svg>
	</span>
	<span class="text-lg font-bold tracking-[-0.01em] text-foreground max-md:text-base">Onyx</span>
</a>
