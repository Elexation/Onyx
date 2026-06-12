<script lang="ts">
	import { Toaster as Sonner, type ToasterProps as SonnerProps } from "svelte-sonner";
	import Loader2Icon from '@lucide/svelte/icons/loader-2';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import OctagonXIcon from '@lucide/svelte/icons/octagon-x';
	import InfoIcon from '@lucide/svelte/icons/info';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

	let { ...restProps }: SonnerProps = $props();
</script>

<Sonner
	class="toaster group"
	position="bottom-center"
	richColors
	style="--normal-bg: var(--color-popover); --normal-text: var(--color-popover-foreground); --normal-border: var(--color-border-2); --success-bg: var(--color-popover); --success-text: var(--color-accent-brand); --success-border: var(--color-border-2); --error-bg: var(--color-popover); --error-text: var(--color-destructive); --error-border: var(--color-border-2); --info-bg: var(--color-popover); --info-text: var(--color-popover-foreground); --info-border: var(--color-border-2); --warning-bg: var(--color-popover); --warning-text: oklch(0.82 0.15 85); --warning-border: var(--color-border-2); --loading-bg: var(--color-popover); --loading-text: var(--color-popover-foreground); --loading-border: var(--color-border-2); --border-radius: 12px; --width: 380px;"
	{...restProps}
>
	{#snippet loadingIcon()}
		<Loader2Icon class="size-4 animate-spin" />
	{/snippet}
	{#snippet successIcon()}
		<CircleCheckIcon class="size-4" />
	{/snippet}
	{#snippet errorIcon()}
		<OctagonXIcon class="size-4" />
	{/snippet}
	{#snippet infoIcon()}
		<InfoIcon class="size-4" />
	{/snippet}
	{#snippet warningIcon()}
		<TriangleAlertIcon class="size-4" />
	{/snippet}
</Sonner>
