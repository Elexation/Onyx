<script lang="ts">
	import { page } from "$app/state";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Toaster } from "$lib/components/ui/sonner/index.js";
	import { Download, FolderOpen, FileIcon, Lock, Eye } from "lucide-svelte";
	import type { FileInfo } from "$lib/types.js";
	import { canPreview } from "$lib/preview.js";
	import PreviewModal from "$lib/components/preview/PreviewModal.svelte";

	const token = $derived(page.params.token);

	let loading = $state(true);
	let error = $state("");
	let passwordRequired = $state(false);
	let password = $state("");
	let verifying = $state(false);
	let verifyError = $state("");

	let fileName = $state("");
	let isDir = $state(false);
	let mimeType = $state("");
	let fileSize = $state(0);
	let items = $state<FileInfo[]>([]);

	let showPreview = $state(false);

	const fileInfo: FileInfo = $derived({ name: fileName, path: "", isDir: false, size: fileSize, modTime: 0, mimeType });
	const previewable = $derived(!isDir && fileName && canPreview(fileInfo));

	function rawUrl() {
		return `/api/public/s/${token}/raw`;
	}

	$effect(() => {
		if (token) loadShare();
	});

	async function loadShare() {
		loading = true;
		error = "";
		try {
			const res = await fetch(`/api/public/s/${token}`);
			if (!res.ok) {
				const data = await res.json().catch(() => ({ error: "Not found" }));
				error = data.error || "Share not found";
				return;
			}
			const data = await res.json();
			if (data.passwordRequired) {
				passwordRequired = true;
				isDir = data.isDir;
				return;
			}
			applyData(data);
		} catch {
			error = "Failed to load share";
		} finally {
			loading = false;
		}
	}

	async function verifyPassword(e: SubmitEvent) {
		e.preventDefault();
		verifying = true;
		verifyError = "";
		try {
			const res = await fetch(`/api/public/s/${token}/verify`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ password }),
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({ error: "Verification failed" }));
				verifyError = data.error || "Incorrect password";
				password = "";
				return;
			}
			const data = await res.json();
			passwordRequired = false;
			applyData(data);
		} catch {
			verifyError = "Verification failed";
		} finally {
			verifying = false;
		}
	}

	function applyData(data: any) {
		fileName = data.fileName;
		isDir = data.isDir;
		mimeType = data.mimeType || "";
		fileSize = data.size || 0;
		items = data.items || [];
	}

	function downloadUrl(subPath?: string) {
		if (subPath) {
			return `/api/public/s/${token}/dl/${subPath}`;
		}
		return `/api/public/s/${token}/dl`;
	}

	function formatSize(bytes: number): string {
		if (bytes === 0) return "0 B";
		const units = ["B", "KB", "MB", "GB"];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	{#if loading}
		<Card.Root class="w-full max-w-md">
			<Card.Content class="py-8 text-center text-muted-foreground">
				Loading...
			</Card.Content>
		</Card.Root>
	{:else if error}
		<Card.Root class="w-full max-w-md">
			<Card.Content class="py-8 text-center text-muted-foreground">
				{error}
			</Card.Content>
		</Card.Root>
	{:else if passwordRequired}
		<Card.Root class="w-full max-w-sm">
			<Card.Header>
				<Card.Title class="flex items-center gap-2 text-lg">
					<Lock class="size-4" />
					Password Required
				</Card.Title>
				<Card.Description>This share is password protected.</Card.Description>
			</Card.Header>
			<Card.Content>
				<form onsubmit={verifyPassword} class="grid gap-4">
					<div class="grid gap-2">
						<Label for="share-pw">Password</Label>
						<Input
							id="share-pw"
							type="password"
							bind:value={password}
							required
							autofocus
						/>
					</div>
					{#if verifyError}
						<p class="text-sm text-destructive">{verifyError}</p>
					{/if}
					<Button type="submit" class="w-full" disabled={verifying}>
						{verifying ? "Verifying..." : "Unlock"}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	{:else if isDir}
		<Card.Root class="w-full max-w-lg">
			<Card.Header>
				<Card.Title class="flex items-center gap-2 text-lg">
					<FolderOpen class="size-4" />
					{fileName}
				</Card.Title>
				<Card.Description>{items.length} item{items.length !== 1 ? "s" : ""}</Card.Description>
			</Card.Header>
			<Card.Content class="flex flex-col gap-4">
				<a href={`/api/public/s/${token}/zip`} class="block">
					<Button variant="secondary" class="w-full">
						<Download class="mr-2 size-4" />
						Download All
					</Button>
				</a>
				{#if items.length === 0}
					<p class="text-sm text-muted-foreground">This folder is empty.</p>
				{:else}
					<div class="flex flex-col divide-y divide-border">
						{#each items as item}
							{#if item.isDir}
								<div class="flex items-center gap-3 py-2 text-sm text-muted-foreground">
									<FolderOpen class="size-4 shrink-0" />
									<span class="truncate">{item.name}</span>
								</div>
							{:else}
								<a
									href={downloadUrl(item.path)}
									class="flex items-center gap-3 py-2 text-sm transition-colors hover:text-foreground"
								>
									<FileIcon class="size-4 shrink-0 text-muted-foreground" />
									<span class="truncate">{item.name}</span>
									<span class="ml-auto shrink-0 text-xs text-muted-foreground">{formatSize(item.size)}</span>
									<Download class="size-3.5 shrink-0 text-muted-foreground" />
								</a>
							{/if}
						{/each}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	{:else}
		<Card.Root class="w-full max-w-sm">
			<Card.Header>
				<Card.Title class="flex items-center gap-2 text-lg">
					<FileIcon class="size-4" />
					{fileName}
				</Card.Title>
			</Card.Header>
			<Card.Content>
				<div class="flex flex-col gap-2">
					{#if previewable}
						<Button class="w-full" onclick={() => showPreview = true}>
							<Eye class="mr-2 size-4" />
							Preview
						</Button>
					{/if}
					<a href={downloadUrl()} class="block">
						<Button variant={previewable ? "secondary" : "default"} class="w-full">
							<Download class="mr-2 size-4" />
							Download
						</Button>
					</a>
				</div>
			</Card.Content>
		</Card.Root>
	{/if}
</div>

{#if showPreview}
	<PreviewModal
		file={fileInfo}
		items={[]}
		onclose={() => showPreview = false}
		url={rawUrl()}
		downloadUrl={downloadUrl()}
	/>
{/if}
<Toaster theme="dark" />
