<script lang="ts">
	interface Props {
		name?: string;
	}
	let { name = "admin" }: Props = $props();
	const initial = $derived((name[0] ?? "a").toUpperCase());
</script>

<div
	class="hidden items-center gap-2 rounded-full border border-border bg-background py-1 pl-1 pr-2.5 text-[13px] lg:flex"
	aria-label="Signed in as {name}"
>
	<span
		class="inline-flex size-6 items-center justify-center rounded-full bg-accent-brand font-mono text-[11px] font-bold text-accent-brand-foreground"
		aria-hidden="true"
	>
		{initial}
	</span>
	<span class="font-mono text-muted-foreground">{name}</span>
</div>
