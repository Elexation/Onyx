<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { getSettings, updateSettings, changePassword } from "$lib/api/settings";
	import { shareCount } from "$lib/api/shares";
	import { versionCount } from "$lib/api/versions";
	import { listTokens, revokeToken } from "$lib/api/tokens";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { trashEnabled } from "$lib/stores/trashEnabled.svelte.js";
	import { versioningEnabled } from "$lib/stores/versioningEnabled.svelte.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import TokenCreateDialog from "$lib/components/dialogs/TokenCreateDialog.svelte";
	import type { PersonalAccessToken, TokenScope } from "$lib/types.js";
	import {
		History,
		Trash2,
		Link2,
		Upload,
		Play,
		Shield,
		KeyRound,
	} from "lucide-svelte";
	import type { Snippet } from "svelte";

	type IconComponent = typeof History;

	const MIN_PASSWORD_LENGTH = 8;

	type SectionKey =
		| "versioning"
		| "trash"
		| "sharing"
		| "uploads"
		| "playback"
		| "security"
		| "tokens";

	const sections: { key: SectionKey; label: string; icon: IconComponent; desc: string }[] = [
		{
			key: "versioning",
			label: "Versioning",
			icon: History,
			desc: "Keep previous versions of files on save.",
		},
		{
			key: "trash",
			label: "Trash",
			icon: Trash2,
			desc: "Hold deleted files for recovery before permanent removal.",
		},
		{
			key: "sharing",
			label: "Sharing",
			icon: Link2,
			desc: "Allow creating public share links for files.",
		},
		{
			key: "uploads",
			label: "Uploads",
			icon: Upload,
			desc: "Limits applied to incoming file uploads.",
		},
		{
			key: "playback",
			label: "Playback",
			icon: Play,
			desc: "Defaults for in-browser video playback.",
		},
		{
			key: "security",
			label: "Security",
			icon: Shield,
			desc: "Session lifetime and admin password.",
		},
		{
			key: "tokens",
			label: "Tokens",
			icon: KeyRound,
			desc: "Personal access tokens for scripts and automation.",
		},
	];

	const caps: Record<string, { min: number; max: number; label: string }> = {
		"versions.max_count": { min: 0, max: 100, label: "Max versions" },
		"versions.max_age": { min: 0, max: 8760, label: "Max version age" },
		"versions.max_file_size": { min: 0, max: 20480, label: "Max file size to version" },
		"versions.max_storage": { min: 0, max: 20480, label: "Max version storage" },
		"trash.purge_age": { min: 0, max: 8760, label: "Trash purge age" },
		"trash.max_size": { min: 0, max: 102400, label: "Max trash size" },
		"session.lifetime": { min: 1, max: 720, label: "Session lifetime" },
		"upload.max_size": { min: 0, max: 102400, label: "Max file size" },
	};

	let section = $state<SectionKey>("versioning");
	let settings = $state<Record<string, string>>({});
	let loading = $state(true);

	let currentPassword = $state("");
	let newPassword = $state("");
	let confirmPassword = $state("");
	let changingPassword = $state(false);

	let shareDisableConfirmOpen = $state(false);
	let versionDisableConfirmOpen = $state(false);
	let versioningChecked = $state(false);
	let sharingChecked = $state(false);
	let debounceTimers: Record<string, ReturnType<typeof setTimeout>> = {};

	let tokens = $state<PersonalAccessToken[]>([]);
	let tokenMax = $state(50);
	let tokensLoading = $state(false);
	let tokenCreateOpen = $state(false);
	let tokenRevokeConfirmOpen = $state(false);
	let tokenToRevoke = $state<PersonalAccessToken | null>(null);

	let navStripEl = $state<HTMLElement | null>(null);

	const currentSection = $derived(sections.find((s) => s.key === section)!);

	function selectSection(next: SectionKey) {
		section = next;
		if (next === "tokens" && tokens.length === 0 && !tokensLoading) loadTokens();
	}

	$effect(() => {
		const target = section;
		if (!navStripEl) return;
		const btn = navStripEl.querySelector<HTMLElement>(`[data-section="${target}"]`);
		btn?.scrollIntoView({ inline: "center", block: "nearest", behavior: "smooth" });
	});

	onMount(async () => {
		try {
			settings = await getSettings();
			versioningChecked = settings["versions.enabled"] === "true";
			sharingChecked = settings["shares.enabled"] === "true";
		} catch {
			toast.error("Failed to load settings");
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		for (const t of Object.values(debounceTimers)) clearTimeout(t);
	});

	function save(key: string, value: string) {
		settings[key] = value;
		if (debounceTimers[key]) clearTimeout(debounceTimers[key]);
		debounceTimers[key] = setTimeout(async () => {
			try {
				const result = await updateSettings({ [key]: value });
				if (result.errors && Object.keys(result.errors).length > 0) {
					toast.error(Object.values(result.errors)[0]);
				} else {
					toast.success("Setting saved");
				}
			} catch {
				toast.error("Failed to save setting");
			}
		}, 500);
	}

	function toggleBool(key: string, checked: boolean) {
		save(key, checked ? "true" : "false");
		if (key === "shares.enabled") {
			sharesEnabled.set(checked);
			sharingChecked = checked;
		}
		if (key === "versions.enabled") {
			versioningEnabled.set(checked);
			versioningChecked = checked;
		}
		if (key === "trash.enabled") {
			trashEnabled.set(checked);
		}
	}

	async function handleShareToggle(checked: boolean) {
		if (checked) {
			toggleBool("shares.enabled", true);
			return;
		}
		try {
			const res = await shareCount();
			if (res.count > 0) {
				shareDisableConfirmOpen = true;
				return;
			}
		} catch {}
		toggleBool("shares.enabled", false);
	}

	function confirmDisableSharing() {
		shareDisableConfirmOpen = false;
		toggleBool("shares.enabled", false);
	}

	function cancelDisableSharing() {
		shareDisableConfirmOpen = false;
		sharingChecked = true;
	}

	async function handleVersionToggle(checked: boolean) {
		if (checked) {
			toggleBool("versions.enabled", true);
			return;
		}
		try {
			const res = await versionCount();
			if (res.count > 0) {
				versionDisableConfirmOpen = true;
				return;
			}
		} catch {}
		toggleBool("versions.enabled", false);
	}

	function confirmDisableVersioning() {
		versionDisableConfirmOpen = false;
		toggleBool("versions.enabled", false);
	}

	function cancelDisableVersioning() {
		versionDisableConfirmOpen = false;
		versioningChecked = true;
	}

	function validateAndSaveInt(key: string, raw: string) {
		const n = parseInt(raw);
		const cap = caps[key];
		if (!cap) return;
		if (isNaN(n)) {
			toast.error(`${cap.label} must be a whole number`);
			return;
		}
		if (n < cap.min) {
			toast.error(`${cap.label} must be at least ${cap.min}`);
			return;
		}
		if (n > cap.max) {
			toast.error(`${cap.label} cannot exceed ${cap.max.toLocaleString()}`);
			return;
		}
		save(key, String(n));
	}

	function validateAndSaveDuration(key: string, raw: string) {
		const n = parseInt(raw);
		const cap = caps[key];
		if (!cap) return;
		if (isNaN(n)) {
			toast.error(`${cap.label} must be a whole number`);
			return;
		}
		if (n < cap.min) {
			toast.error(`${cap.label} must be at least ${cap.min} hour`);
			return;
		}
		if (n > cap.max) {
			toast.error(`${cap.label} cannot exceed ${cap.max.toLocaleString()} hours`);
			return;
		}
		save(key, `${n}h`);
	}

	function validateAndSaveMB(key: string, raw: string) {
		const n = parseInt(raw);
		const cap = caps[key];
		if (!cap) return;
		if (isNaN(n)) {
			toast.error(`${cap.label} must be a whole number`);
			return;
		}
		if (n < cap.min) {
			toast.error(`${cap.label} must be at least ${cap.min}`);
			return;
		}
		if (n > cap.max) {
			toast.error(`${cap.label} cannot exceed ${cap.max.toLocaleString()} MB`);
			return;
		}
		save(key, String(n * 1024 * 1024));
	}

	function durationToHours(val: string): number {
		if (!val) return 0;
		const match = val.match(/^(\d+)h$/);
		return match ? parseInt(match[1]) : 0;
	}

	function bytesToMB(val: string): number {
		const n = parseInt(val);
		if (isNaN(n) || n === 0) return 0;
		return Math.round(n / (1024 * 1024));
	}

	const qualityOptions = [
		{ value: "0", label: "Unlimited (source)" },
		{ value: "2160", label: "2160p" },
		{ value: "1440", label: "1440p" },
		{ value: "1080", label: "1080p" },
		{ value: "720", label: "720p" },
		{ value: "480", label: "480p" },
	];

	function qualityLabel(value: string): string {
		return qualityOptions.find((o) => o.value === value)?.label ?? "1080p";
	}

	async function loadTokens() {
		tokensLoading = true;
		try {
			const res = await listTokens();
			tokens = res.tokens ?? [];
			tokenMax = res.max;
		} catch {
			toast.error("Failed to load tokens");
		} finally {
			tokensLoading = false;
		}
	}

	function handleTokenCreated(tok: PersonalAccessToken) {
		tokens = [tok, ...tokens];
	}

	function askRevokeToken(tok: PersonalAccessToken) {
		tokenToRevoke = tok;
		tokenRevokeConfirmOpen = true;
	}

	async function confirmRevokeToken() {
		if (!tokenToRevoke) return;
		const id = tokenToRevoke.id;
		tokenRevokeConfirmOpen = false;
		tokenToRevoke = null;
		try {
			await revokeToken(id);
			tokens = tokens.filter((t) => t.id !== id);
			toast.success("Token revoked");
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "Failed to revoke token");
		}
	}

	function scopeBadgeLabel(s: TokenScope): string {
		if (s === "read") return "Read-only";
		if (s === "upload") return "Upload + list";
		return "Full access";
	}

	function formatTokenDate(unix: number | undefined): string {
		if (!unix) return "Never";
		return new Date(unix * 1000).toLocaleDateString(undefined, {
			month: "short",
			day: "numeric",
			year: "numeric",
		});
	}

	function formatLastUsed(unix: number | undefined): string {
		if (!unix) return "Never";
		const now = Date.now() / 1000;
		const diff = now - unix;
		if (diff < 60) return "Just now";
		if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
		if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
		return formatTokenDate(unix);
	}

	async function handleChangePassword() {
		if (!currentPassword || !newPassword) {
			toast.error("All password fields are required");
			return;
		}
		if (newPassword.length < MIN_PASSWORD_LENGTH) {
			toast.error(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`);
			return;
		}
		if (newPassword !== confirmPassword) {
			toast.error("New passwords do not match");
			return;
		}
		if (currentPassword === newPassword) {
			toast.error("New password must be different from current password");
			return;
		}
		changingPassword = true;
		try {
			await changePassword(currentPassword, newPassword);
			toast.success("Password changed — other sessions invalidated");
			currentPassword = "";
			newPassword = "";
			confirmPassword = "";
		} catch (e: any) {
			toast.error(e.message || "Failed to change password");
		} finally {
			changingPassword = false;
		}
	}
</script>

{#snippet row(label: string, desc: string, control: Snippet)}
	<div class="flex items-center justify-between gap-6 border-t border-border py-4">
		<div class="min-w-0 flex-1">
			<p class="text-sm font-medium">{label}</p>
			{#if desc}
				<p class="mt-0.5 text-xs text-muted-foreground">{desc}</p>
			{/if}
		</div>
		<div class="shrink-0">{@render control()}</div>
	</div>
{/snippet}

<div class="flex h-full flex-col md:flex-row">
	<aside
		bind:this={navStripEl}
		class="flex shrink-0 flex-row gap-1 overflow-x-auto border-b border-border p-2 [mask-image:linear-gradient(to_right,black_calc(100%_-_24px),transparent)] md:w-[200px] md:flex-col md:gap-0.5 md:overflow-visible md:border-b-0 md:border-r md:p-3 md:[mask-image:none]"
	>
		{#each sections as s (s.key)}
			{@const active = section === s.key}
			<button
				type="button"
				data-section={s.key}
				onclick={() => selectSection(s.key)}
				class="flex shrink-0 cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] font-medium transition-colors
					{active
					? 'bg-muted text-foreground'
					: 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
			>
				<s.icon size={15} strokeWidth={2} />
				{s.label}
			</button>
		{/each}
	</aside>

	<div class="min-w-0 flex-1 overflow-auto">
		<div class="mx-auto max-w-[720px] p-6 md:px-8 md:py-7">
			{#if loading}
				<p class="text-sm text-muted-foreground">Loading settings…</p>
			{:else}
				<header class="mb-5 flex items-start justify-between gap-4">
					<div class="min-w-0">
						<h1 class="text-[22px] font-bold tracking-[-0.01em]">{currentSection.label}</h1>
						<p class="mt-1 text-[13px] text-muted-foreground">{currentSection.desc}</p>
					</div>
					{#if section === "tokens"}
						<Button
							onclick={() => (tokenCreateOpen = true)}
							disabled={tokens.length >= tokenMax}
						>
							Create Token
						</Button>
					{/if}
				</header>

				{#if section === "versioning"}
					{#snippet versioningSwitch()}
						<Switch
							bind:checked={versioningChecked}
							onCheckedChange={(checked: boolean) => handleVersionToggle(checked)}
						/>
					{/snippet}
					{@render row(
						"Enable file versioning",
						"Keep previous versions of files on save.",
						versioningSwitch,
					)}

					{#snippet versionsMaxCount()}
						<Input
							type="number"
							min="0"
							max="100"
							step="1"
							value={settings["versions.max_count"] ?? "10"}
							onchange={(e) => validateAndSaveInt("versions.max_count", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Maximum versions per file",
						"How many old versions to keep per file. 0 = unlimited, max 100.",
						versionsMaxCount,
					)}

					{#snippet versionsMaxAge()}
						<Input
							type="number"
							min="0"
							max="8760"
							step="1"
							value={durationToHours(settings["versions.max_age"] ?? "2160h")}
							onchange={(e) => validateAndSaveDuration("versions.max_age", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Maximum version age (hours)",
						"Discard versions older than this. 0 = never expire, max 8,760 (1 year). Default 2,160 (90 days).",
						versionsMaxAge,
					)}

					{#snippet versionsMaxFileSize()}
						<Input
							type="number"
							min="0"
							max="20480"
							step="1"
							value={bytesToMB(settings["versions.max_file_size"] ?? "1073741824")}
							onchange={(e) => validateAndSaveMB("versions.max_file_size", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Maximum file size to version (MB)",
						"Files larger than this are not versioned. 0 = unlimited, max 20,480 (20 GB). Default 1,024 (1 GB).",
						versionsMaxFileSize,
					)}

					{#snippet versionsMaxStorage()}
						<Input
							type="number"
							min="0"
							max="20480"
							step="1"
							value={bytesToMB(settings["versions.max_storage"] ?? "0")}
							onchange={(e) => validateAndSaveMB("versions.max_storage", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Maximum version storage (MB)",
						"Total storage budget for versions. Oldest are purged when exceeded. 0 = unlimited, max 20,480 (20 GB).",
						versionsMaxStorage,
					)}
				{/if}

				{#if section === "trash"}
					{#snippet trashEnabledSwitch()}
						<Switch
							checked={settings["trash.enabled"] === "true"}
							onCheckedChange={(checked: boolean) => toggleBool("trash.enabled", checked)}
						/>
					{/snippet}
					{@render row(
						"Enable trash",
						"Move deleted files to trash instead of permanent deletion.",
						trashEnabledSwitch,
					)}

					{#snippet trashPurgeAge()}
						<Input
							type="number"
							min="0"
							max="8760"
							step="1"
							value={durationToHours(settings["trash.purge_age"] ?? "720h")}
							onchange={(e) => validateAndSaveDuration("trash.purge_age", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Auto-purge after (hours)",
						"Automatically delete trashed items after this long. 0 = never purge, max 8,760 (1 year). Default 720 (30 days).",
						trashPurgeAge,
					)}

					{#snippet trashMaxSize()}
						<Input
							type="number"
							min="0"
							max="102400"
							step="1"
							value={bytesToMB(settings["trash.max_size"] ?? "0")}
							onchange={(e) => validateAndSaveMB("trash.max_size", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Maximum trash size (MB)",
						"Total storage budget for trash. Oldest items are purged when exceeded. 0 = unlimited, max 102,400 (100 GB).",
						trashMaxSize,
					)}
				{/if}

				{#if section === "sharing"}
					{#snippet sharingSwitch()}
						<Switch
							bind:checked={sharingChecked}
							onCheckedChange={(checked: boolean) => handleShareToggle(checked)}
						/>
					{/snippet}
					{@render row(
						"Enable sharing",
						"Allow creating public share links for files.",
						sharingSwitch,
					)}
				{/if}

				{#if section === "uploads"}
					{#snippet uploadMaxSize()}
						<Input
							type="number"
							min="0"
							max="102400"
							step="1"
							value={bytesToMB(settings["upload.max_size"] ?? "0")}
							onchange={(e) => validateAndSaveMB("upload.max_size", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Maximum file size (MB)",
						"Reject uploads larger than this. 0 = unlimited, max 102,400 (100 GB).",
						uploadMaxSize,
					)}
				{/if}

				{#if section === "playback"}
					{#snippet playbackQuality()}
						<Select.Root
							type="single"
							value={settings["playback.default_quality_ceiling"] ?? "1080"}
							onValueChange={(v) => save("playback.default_quality_ceiling", v)}
						>
							<Select.Trigger class="w-[180px]">
								{qualityLabel(settings["playback.default_quality_ceiling"] ?? "1080")}
							</Select.Trigger>
							<Select.Content>
								{#each qualityOptions as opt}
									<Select.Item value={opt.value}>{opt.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
					{@render row(
						"Default quality ceiling",
						"Caps the highest rendition produced when transcoding videos. Viewers can still pick a lower quality manually. “Unlimited” encodes up to source resolution.",
						playbackQuality,
					)}
				{/if}

				{#if section === "security"}
					{#snippet sessionLifetime()}
						<Input
							type="number"
							min="1"
							max="720"
							step="1"
							value={durationToHours(settings["session.lifetime"] ?? "720h")}
							onchange={(e) => validateAndSaveDuration("session.lifetime", e.currentTarget.value)}
							class="w-[140px]"
						/>
					{/snippet}
					{@render row(
						"Session lifetime (hours)",
						"How long a sign-in stays valid. 1–720 hours (30 days). Default 720. Only affects new sessions.",
						sessionLifetime,
					)}

					<div class="mt-8 border-t border-border pt-6">
						<h3 class="text-sm font-medium">Change password</h3>
						<p class="mt-0.5 text-xs text-muted-foreground">
							Other sessions are signed out when the password changes.
						</p>
						<div class="mt-4 max-w-xs space-y-3">
							<div class="space-y-1">
								<Label for="current-password">Current password</Label>
								<Input
									id="current-password"
									type="password"
									bind:value={currentPassword}
								/>
							</div>
							<div class="space-y-1">
								<Label for="new-password">New password</Label>
								<Input
									id="new-password"
									type="password"
									placeholder="Minimum {MIN_PASSWORD_LENGTH} characters"
									bind:value={newPassword}
								/>
							</div>
							<div class="space-y-1">
								<Label for="confirm-password">Confirm new password</Label>
								<Input
									id="confirm-password"
									type="password"
									bind:value={confirmPassword}
								/>
							</div>
							<Button
								onclick={handleChangePassword}
								disabled={changingPassword || !currentPassword || !newPassword || !confirmPassword}
							>
								{changingPassword ? "Changing…" : "Change password"}
							</Button>
						</div>
					</div>
				{/if}

				{#if section === "tokens"}
					<div class="border-t border-border pt-4">
						<p class="mb-4 text-xs text-muted-foreground">
							{tokens.length} of {tokenMax} used.
						</p>

						{#if tokensLoading}
							<p class="text-sm text-muted-foreground">Loading tokens…</p>
						{:else if tokens.length === 0}
							<p class="text-sm text-muted-foreground">
								No tokens yet. Create one to authenticate scripts against the Onyx API.
							</p>
						{:else}
							<div class="flex flex-col gap-3">
								{#each tokens as tok (tok.id)}
									<div class="rounded-xl border border-border bg-card p-[14px]">
										<div class="flex items-start justify-between gap-3">
											<div class="min-w-0 flex-1 space-y-1.5">
												<div class="flex items-center gap-2 text-[15px] font-medium">
													<span class="truncate">{tok.name}</span>
													<span class="shrink-0 rounded-[5px] bg-muted px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.02em] text-muted-foreground">
														{scopeBadgeLabel(tok.scope)}
													</span>
												</div>
												<p class="truncate font-mono text-[13px] text-muted-foreground">
													onyx_…{tok.tokenLast8}
												</p>
												<div class="grid grid-cols-1 gap-x-3 gap-y-0.5 font-mono text-[11px] text-muted-foreground md:grid-cols-3">
													<span>Created {formatTokenDate(tok.createdAt)}</span>
													<span>Last used {formatLastUsed(tok.lastUsedAt)}</span>
													<span>Expires {formatTokenDate(tok.expiresAt)}</span>
												</div>
											</div>
											<Button
												variant="ghost"
												size="icon-xs"
												class="shrink-0 cursor-pointer text-muted-foreground hover:text-destructive"
												onclick={() => askRevokeToken(tok)}
												title="Revoke token"
											>
												<Trash2 class="size-4" strokeWidth={2} />
											</Button>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			{/if}
		</div>
	</div>
</div>

<AlertDialog.Root bind:open={shareDisableConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Disable sharing?</AlertDialog.Title>
			<AlertDialog.Description>
				This will delete all existing share links. Anyone with a link will no longer be able to access shared files.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={cancelDisableSharing}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDisableSharing}>
				Disable & Delete Links
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={versionDisableConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Disable versioning?</AlertDialog.Title>
			<AlertDialog.Description>
				This will permanently delete all stored version files. You will not be able to restore previous versions of any file.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={cancelDisableVersioning}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDisableVersioning}>
				Disable & Delete Versions
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={tokenRevokeConfirmOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke token?</AlertDialog.Title>
			<AlertDialog.Description>
				{tokenToRevoke
					? `"${tokenToRevoke.name}" will stop working immediately. Any script using it will fail.`
					: ""}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmRevokeToken}>
				Revoke
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<TokenCreateDialog bind:open={tokenCreateOpen} onCreated={handleTokenCreated} />
