<script lang="ts">
	let {
		text = "Loading…",
		compact = false,
	}: { text?: string; compact?: boolean } = $props();
</script>

<div class="flex items-center justify-center {compact ? 'py-6' : 'py-20'} text-sm text-muted-foreground">
	{text}
</div>
