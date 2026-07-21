<script lang="ts">
	import { Switch as SwitchPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		checked = $bindable(false),
		...restProps
	}: WithoutChildrenOrChild<SwitchPrimitive.RootProps> = $props();
</script>

<SwitchPrimitive.Root
	bind:ref
	bind:checked
	data-slot="switch"
	class={cn(
		"data-[state=checked]:bg-accent-brand data-[state=unchecked]:bg-input/80 focus-visible:border-ring focus-visible:ring-ring/50 peer relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-transparent shadow-xs transition-all outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50 after:absolute after:-inset-x-0.5 after:-inset-y-1.5",
		className
	)}
	{...restProps}
>
	<SwitchPrimitive.Thumb
		data-slot="switch-thumb"
		class={cn(
			"data-[state=unchecked]:bg-foreground data-[state=checked]:bg-accent-brand-foreground pointer-events-none block size-4 rounded-full ring-0 transition-transform data-[state=checked]:translate-x-[calc(100%+2px)] data-[state=unchecked]:translate-x-0 rtl:data-[state=checked]:translate-x-[calc(-100%-4px)]"
		)}
	/>
</SwitchPrimitive.Root>
