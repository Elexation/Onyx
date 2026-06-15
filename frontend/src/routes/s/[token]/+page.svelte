<script lang="ts">
	import { page } from "$app/state";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Toaster } from "$lib/components/ui/sonner/index.js";
	import BrandMark from "$lib/components/BrandMark.svelte";
	import FileIcon from "$lib/components/FileIcon.svelte";
	import { Download, Lock, Eye, Folder } from "lucide-svelte";
	import type { FileInfo } from "$lib/types.js";
	import { canPreview } from "$lib/preview.js";
	import { encodeFilePath } from "$lib/utils";
	import { formatFileSize } from "$lib/utils/format.js";
	import PreviewModal from "$lib/components/preview/PreviewModal.svelte";

	const token = $derived(page.params.token);
	const safeToken = $derived(encodeURIComponent(token ?? ""));

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

	const lastDot = $derived(fileName.lastIndexOf("."));
	const ext = $derived(!isDir && lastDot > 0 ? fileName.slice(lastDot + 1, lastDot + 5).toUpperCase() : null);

	function rawUrl() {
		return `/api/public/s/${safeToken}/raw`;
	}

	$effect(() => {
		if (token) loadShare();
	});

	async function loadShare() {
		loading = true;
		error = "";
		try {
			const res = await fetch(`/api/public/s/${safeToken}`);
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
			const res = await fetch(`/api/public/s/${safeToken}/verify`, {
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
			return `/api/public/s/${safeToken}/dl${encodeFilePath(subPath)}`;
		}
		return `/api/public/s/${safeToken}/dl`;
	}
</script>

<div class="flex min-h-screen flex-col items-center justify-center gap-6 px-4 py-10">
	<BrandMark href={`/s/${safeToken}`} />

	{#if loading}
		<Card.Root class="w-full max-w-sm">
			<Card.Content class="py-10 text-center text-[15px] text-muted-foreground">
				Loading…
			</Card.Content>
		</Card.Root>
	{:else if error}
		<Card.Root class="w-full max-w-sm">
			<Card.Content class="py-10 text-center text-[15px] text-muted-foreground">
				{error}
			</Card.Content>
		</Card.Root>
	{:else if passwordRequired}
		<Card.Root class="w-full max-w-sm">
			<Card.Header>
				<Card.Title class="flex items-center gap-2 text-lg font-bold tracking-[-0.01em]">
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
					<Button type="submit" size="lg" class="w-full" disabled={verifying}>
						{verifying ? "Verifying…" : "Unlock"}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	{:else if isDir}
		<div class="flex w-full max-w-3xl flex-col gap-4">
			<div class="flex flex-col gap-3 rounded-xl border border-border bg-card p-[14px] md:flex-row md:items-center md:justify-between">
				<div class="flex min-w-0 items-center gap-3">
					<Folder class="size-5 shrink-0 text-accent-brand" strokeWidth={2} />
					<div class="min-w-0">
						<div class="truncate text-[15px] font-medium">{fileName}</div>
						<div class="font-mono text-[13px] text-muted-foreground">
							{items.length} item{items.length !== 1 ? "s" : ""}
						</div>
					</div>
				</div>
				<a href={`/api/public/s/${safeToken}/zip`} class="shrink-0">
					<Button class="w-full md:w-auto">
						<Download class="mr-2 size-4" />
						Download All
					</Button>
				</a>
			</div>

			{#if items.length === 0}
				<div class="flex items-center justify-center rounded-xl border border-border bg-card py-20 text-[15px] text-muted-foreground">
					This folder is empty.
				</div>
			{:else}
				<div class="flex flex-col overflow-hidden rounded-xl border border-border bg-card">
					<div class="hidden border-b border-border px-[14px] py-2.5 font-mono text-[11px] font-semibold tracking-wider text-muted-foreground uppercase md:grid md:grid-cols-[minmax(0,1fr)_100px_40px] md:gap-3">
						<div>Name</div>
						<div class="text-right">Size</div>
						<div></div>
					</div>
					{#each items as item (item.path)}
						{@const itemLastDot = item.name.lastIndexOf(".")}
						{@const itemExt = !item.isDir && itemLastDot > 0 ? item.name.slice(itemLastDot + 1, itemLastDot + 5).toUpperCase() : null}
						{#if item.isDir}
							<div class="grid grid-cols-[1fr_auto] items-center gap-3 border-b border-border px-[14px] py-3.5 text-muted-foreground last:border-b-0 md:grid-cols-[minmax(0,1fr)_100px_40px] md:py-[11px]">
								<div class="flex min-w-0 items-center gap-3">
									<FileIcon
										isDir={true}
										class="size-6 shrink-0 text-accent-brand md:size-5"
										strokeWidth={1.4}
									/>
									<span class="truncate text-[15px]">{item.name}</span>
								</div>
								<div class="hidden text-right font-mono text-[13px] md:block">—</div>
								<div class="hidden md:block"></div>
							</div>
						{:else}
							<a
								href={downloadUrl(item.path)}
								class="group grid grid-cols-[1fr_auto] items-center gap-3 border-b border-border px-[14px] py-3.5 transition-colors last:border-b-0 hover:bg-muted md:grid-cols-[minmax(0,1fr)_100px_40px] md:py-[11px]"
							>
								<div class="flex min-w-0 items-center gap-3">
									<FileIcon
										mimeType={item.mimeType}
										class="size-6 shrink-0 text-muted-foreground md:size-5"
										strokeWidth={1.4}
									/>
									<span class="truncate text-[15px] font-medium">{item.name}</span>
									{#if itemExt}
										<span class="shrink-0 rounded-[5px] bg-muted px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] text-muted-foreground">
											{itemExt}
										</span>
									{/if}
								</div>
								<div class="hidden text-right font-mono text-[13px] text-muted-foreground md:block">
									{formatFileSize(item.size)}
								</div>
								<div class="hidden items-center justify-end text-muted-foreground transition-colors group-hover:text-foreground md:flex">
									<Download class="size-4" />
								</div>
							</a>
						{/if}
					{/each}
				</div>
			{/if}
		</div>
	{:else}
		<Card.Root class="w-full max-w-sm">
			<Card.Content class="flex flex-col gap-5 py-6">
				<div class="flex flex-col items-center gap-3 text-center">
					<div class="flex size-16 items-center justify-center rounded-xl bg-muted text-muted-foreground">
						<FileIcon {mimeType} class="size-8" strokeWidth={1.2} />
					</div>
					<div class="flex min-w-0 flex-col items-center gap-1">
						<div class="w-full truncate text-[15px] font-medium">{fileName}</div>
						<div class="flex items-center gap-2 font-mono text-[13px] text-muted-foreground">
							{#if ext}
								<span class="rounded-[5px] bg-muted px-1.5 py-0.5 text-[11px] font-medium tracking-[0.02em]">
									{ext}
								</span>
							{/if}
							<span>{formatFileSize(fileSize)}</span>
						</div>
					</div>
				</div>
				<div class="flex flex-col gap-2">
					{#if previewable}
						<Button size="lg" class="w-full" onclick={() => showPreview = true}>
							<Eye class="mr-2 size-4" />
							Preview
						</Button>
					{/if}
					<a href={downloadUrl()} class="block">
						<Button variant={previewable ? "outline" : "default"} size="lg" class="w-full">
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
