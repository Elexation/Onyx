<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { createShare } from "$lib/api/shares.js";
	import { toast } from "svelte-sonner";
	import { Check, Copy, Link } from "lucide-svelte";

	let {
		open = $bindable(false),
		path,
		isDir,
	}: {
		open: boolean;
		path: string;
		isDir: boolean;
	} = $props();

	let expiresIn = $state("168h");
	let usePassword = $state(false);
	let password = $state("");
	let submitting = $state(false);
	let shareUrl = $state("");
	let copied = $state(false);

	const expiryOptions = [
		{ value: "1h", label: "1 hour" },
		{ value: "24h", label: "1 day" },
		{ value: "168h", label: "7 days" },
		{ value: "720h", label: "30 days" },
		{ value: "2160h", label: "90 days" },
	];

	const selectedLabel = $derived(
		expiryOptions.find((o) => o.value === expiresIn)?.label ?? "7 days"
	);

	$effect(() => {
		if (open) {
			expiresIn = "168h";
			usePassword = false;
			password = "";
			shareUrl = "";
			copied = false;
		}
	});

	async function submit() {
		submitting = true;
		try {
			const result = await createShare({
				path,
				isDir,
				expiresIn,
				password: usePassword ? password : undefined,
			});
			shareUrl = `${window.location.origin}/s/${result.token}`;
			toast.success("Share link created");
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to create share");
		} finally {
			submitting = false;
		}
	}

	async function copyUrl() {
		await navigator.clipboard.writeText(shareUrl);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<Link class="size-4" />
				Share
			</Dialog.Title>
			<Dialog.Description class="truncate">
				{path}
			</Dialog.Description>
		</Dialog.Header>

		{#if shareUrl}
			<div class="flex flex-col gap-3">
				<Label>Share URL</Label>
				<div class="flex gap-2">
					<Input value={shareUrl} readonly class="font-mono text-xs" />
					<Button variant="outline" size="icon" onclick={copyUrl} class="shrink-0">
						{#if copied}
							<Check class="size-4" />
						{:else}
							<Copy class="size-4" />
						{/if}
					</Button>
				</div>
			</div>
			<Dialog.Footer>
				<Button onclick={() => (open = false)}>Done</Button>
			</Dialog.Footer>
		{:else}
			<div class="flex flex-col gap-4">
				<div class="flex flex-col gap-2">
					<Label>Expiration</Label>
					<Select.Root type="single" bind:value={expiresIn}>
						<Select.Trigger>
							{selectedLabel}
						</Select.Trigger>
						<Select.Content>
							{#each expiryOptions as opt}
								<Select.Item value={opt.value}>{opt.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>

				<div class="flex items-center justify-between">
					<Label>Password protection</Label>
					<Switch bind:checked={usePassword} />
				</div>

				{#if usePassword}
					<Input
						type="password"
						bind:value={password}
						placeholder="Enter password"
					/>
				{/if}
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)}>Cancel</Button>
				<Button onclick={submit} disabled={submitting || (usePassword && !password)}>
					Create Link
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
