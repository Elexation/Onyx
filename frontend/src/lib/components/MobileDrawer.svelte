<script lang="ts">
	import type { Snippet } from "svelte";

	interface Props {
		open?: boolean;
		children: Snippet;
	}
	let { open = $bindable(false), children }: Props = $props();

	function onKeyDown(e: KeyboardEvent) {
		if (e.key === "Escape" && open) {
			open = false;
		}
	}
</script>

<svelte:window onkeydown={onKeyDown} />

<button
	type="button"
	class="absolute inset-0 z-[14] cursor-default bg-black/50 backdrop-blur-sm transition-opacity duration-200 md:hidden {open
		? 'pointer-events-auto opacity-100'
		: 'pointer-events-none opacity-0'}"
	aria-label="Close navigation"
	aria-hidden={!open}
	tabindex={open ? 0 : -1}
	onclick={() => (open = false)}
></button>

<div
	class="shrink-0 md:relative md:translate-x-0 max-md:absolute max-md:inset-y-0 max-md:left-0 max-md:z-[15] max-md:transition-transform max-md:duration-[240ms] max-md:ease-[cubic-bezier(0.2,0.8,0.2,1)] {open
		? 'max-md:translate-x-0'
		: 'max-md:-translate-x-full'}"
>
	{@render children()}
</div>
