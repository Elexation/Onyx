<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { getSettings, updateSettings, changePassword } from "$lib/api/settings";
	import { shareCount } from "$lib/api/shares";
	import { versionCount } from "$lib/api/versions";
	import { listTokens, revokeToken } from "$lib/api/tokens";
	import { sharesEnabled } from "$lib/stores/sharesEnabled.svelte.js";
	import { versioningEnabled } from "$lib/stores/versioningEnabled.svelte.js";
	import { Tabs, TabsList, TabsTrigger, TabsContent } from "$lib/components/ui/tabs/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Separator } from "$lib/components/ui/separator/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import * as AlertDialog from "$lib/components/ui/alert-dialog/index.js";
	import TokenCreateDialog from "$lib/components/dialogs/TokenCreateDialog.svelte";
	import type { PersonalAccessToken, TokenScope } from "$lib/types.js";
	import { Trash2 } from "lucide-svelte";

	const MIN_PASSWORD_LENGTH = 8;

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

	let settings = $state<Record<string, string>>({});
	let loading = $state(true);

	let currentPassword = $state("");
	let newPassword = $state("");
	let confirmPassword = $state("");
	let changingPassword = $state(false);

	let shareDisableConfirmOpen = $state(false);
	let versionDisableConfirmOpen = $state(false);
	let debounceTimers: Record<string, ReturnType<typeof setTimeout>> = {};

	let tokens = $state<PersonalAccessToken[]>([]);
	let tokenMax = $state(50);
	let tokensLoading = $state(false);
	let tokenCreateOpen = $state(false);
	let tokenRevokeConfirmOpen = $state(false);
	let tokenToRevoke = $state<PersonalAccessToken | null>(null);

	onMount(async () => {
		try {
			settings = await getSettings();
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
		}
		if (key === "versions.enabled") {
			versioningEnabled.set(checked);
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

<div class="mx-auto max-w-2xl p-6">
	<h1 class="mb-6 text-2xl font-semibold">Settings</h1>

	{#if loading}
		<p class="text-muted-foreground">Loading settings…</p>
	{:else}
		<Tabs
			value="versioning"
			onValueChange={(v) => {
				if (v === "tokens" && tokens.length === 0 && !tokensLoading) loadTokens();
			}}
		>
			<TabsList class="mb-6">
				<TabsTrigger value="versioning">Versioning</TabsTrigger>
				<TabsTrigger value="trash">Trash</TabsTrigger>
				<TabsTrigger value="sharing">Sharing</TabsTrigger>
				<TabsTrigger value="uploads">Uploads</TabsTrigger>
				<TabsTrigger value="playback">Playback</TabsTrigger>
				<TabsTrigger value="security">Security</TabsTrigger>
				<TabsTrigger value="tokens">Tokens</TabsTrigger>
			</TabsList>


			<!-- Versioning -->
			<TabsContent value="versioning">
				<div class="space-y-6">
					<div class="flex items-center justify-between">
						<div>
							<p class="text-sm font-medium">Enable file versioning</p>
							<p class="text-sm text-muted-foreground">Keep previous versions of files on save</p>
						</div>
						<Switch
							checked={settings["versions.enabled"] === "true"}
							onCheckedChange={(checked: boolean) => handleVersionToggle(checked)}
						/>
					</div>
					<Separator />
					<div class="space-y-2">
						<Label for="versions-max-count">Maximum versions per file</Label>
						<Input
							id="versions-max-count"
							type="number"
							min="0"
							max="100"
							step="1"
							value={settings["versions.max_count"] ?? "10"}
							onchange={(e) => validateAndSaveInt("versions.max_count", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 100</p>
					</div>
					<div class="space-y-2">
						<Label for="versions-max-age">Maximum version age (hours)</Label>
						<Input
							id="versions-max-age"
							type="number"
							min="0"
							max="8760"
							step="1"
							value={durationToHours(settings["versions.max_age"] ?? "2160h")}
							onchange={(e) => validateAndSaveDuration("versions.max_age", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = never expire. Max: 8,760 hours (1 year). Default: 2,160 (90 days)</p>
					</div>
					<div class="space-y-2">
						<Label for="versions-max-file-size">Maximum file size to version (MB)</Label>
						<Input
							id="versions-max-file-size"
							type="number"
							min="0"
							max="20480"
							step="1"
							value={bytesToMB(settings["versions.max_file_size"] ?? "1073741824")}
							onchange={(e) => validateAndSaveMB("versions.max_file_size", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 20,480 MB (20 GB). Default: 1,024 (1 GB). Files larger than this are not versioned.</p>
					</div>
					<div class="space-y-2">
						<Label for="versions-max-storage">Maximum version storage (MB)</Label>
						<Input
							id="versions-max-storage"
							type="number"
							min="0"
							max="20480"
							step="1"
							value={bytesToMB(settings["versions.max_storage"] ?? "0")}
							onchange={(e) => validateAndSaveMB("versions.max_storage", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 20,480 MB (20 GB). Oldest versions are purged when exceeded.</p>
					</div>
				</div>
			</TabsContent>

			<!-- Trash -->
			<TabsContent value="trash">
				<div class="space-y-6">
					<div class="flex items-center justify-between">
						<div>
							<p class="text-sm font-medium">Enable trash</p>
							<p class="text-sm text-muted-foreground">Move deleted files to trash instead of permanent deletion</p>
						</div>
						<Switch
							checked={settings["trash.enabled"] === "true"}
							onCheckedChange={(checked: boolean) => toggleBool("trash.enabled", checked)}
						/>
					</div>
					<Separator />
					<div class="space-y-2">
						<Label for="trash-purge-age">Auto-purge after (hours)</Label>
						<Input
							id="trash-purge-age"
							type="number"
							min="0"
							max="8760"
							step="1"
							value={durationToHours(settings["trash.purge_age"] ?? "720h")}
							onchange={(e) => validateAndSaveDuration("trash.purge_age", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = never purge. Max: 8,760 hours (1 year). Default: 720 (30 days)</p>
					</div>
					<div class="space-y-2">
						<Label for="trash-max-size">Maximum trash size (MB)</Label>
						<Input
							id="trash-max-size"
							type="number"
							min="0"
							max="102400"
							step="1"
							value={bytesToMB(settings["trash.max_size"] ?? "0")}
							onchange={(e) => validateAndSaveMB("trash.max_size", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 102,400 MB (100 GB). Oldest items are purged when exceeded.</p>
					</div>
				</div>
			</TabsContent>

			<!-- Sharing -->
			<TabsContent value="sharing">
				<div class="space-y-6">
					<div class="flex items-center justify-between">
						<div>
							<p class="text-sm font-medium">Enable sharing</p>
							<p class="text-sm text-muted-foreground">Allow creating public share links</p>
						</div>
						<Switch
							checked={settings["shares.enabled"] === "true"}
							onCheckedChange={(checked: boolean) => handleShareToggle(checked)}
						/>
					</div>
				</div>
			</TabsContent>

			<!-- Uploads -->
			<TabsContent value="uploads">
				<div class="space-y-4">
					<div class="space-y-2">
						<Label for="upload-max-size">Maximum file size (MB)</Label>
						<Input
							id="upload-max-size"
							type="number"
							min="0"
							max="102400"
							step="1"
							value={bytesToMB(settings["upload.max_size"] ?? "0")}
							onchange={(e) => validateAndSaveMB("upload.max_size", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 102,400 MB (100 GB)</p>
					</div>
				</div>
			</TabsContent>

			<!-- Playback -->
			<TabsContent value="playback">
				<div class="space-y-6">
					<div class="space-y-2">
						<Label>Default quality ceiling</Label>
						<Select.Root
							type="single"
							value={settings["playback.default_quality_ceiling"] ?? "1080"}
							onValueChange={(v) => save("playback.default_quality_ceiling", v)}
						>
							<Select.Trigger class="max-w-xs">
								{qualityLabel(settings["playback.default_quality_ceiling"] ?? "1080")}
							</Select.Trigger>
							<Select.Content>
								{#each qualityOptions as opt}
									<Select.Item value={opt.value}>{opt.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						<p class="text-xs text-muted-foreground">
							Caps the highest rendition produced when transcoding videos. Viewers can still pick a lower quality manually. "Unlimited" encodes up to the source resolution.
						</p>
					</div>
				</div>
			</TabsContent>

			<!-- Security -->
			<TabsContent value="security">
				<div class="space-y-6">
					<div class="space-y-2">
						<Label for="session-lifetime">Session lifetime (hours)</Label>
						<Input
							id="session-lifetime"
							type="number"
							min="1"
							max="720"
							step="1"
							value={durationToHours(settings["session.lifetime"] ?? "720h")}
							onchange={(e) => validateAndSaveDuration("session.lifetime", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">1–720 hours (30 days). Default: 720 (30 days). Only affects new sessions.</p>
					</div>

					<Separator />

					<div class="space-y-4">
						<h3 class="text-sm font-medium">Change password</h3>
						<div class="max-w-xs space-y-3">
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
				</div>
			</TabsContent>

			<!-- Tokens -->
			<TabsContent value="tokens">
				<div class="space-y-6">
					<div class="flex items-start justify-between gap-4">
						<div>
							<p class="text-sm font-medium">Personal access tokens</p>
							<p class="text-sm text-muted-foreground">
								For authenticating scripts and automation. {tokens.length} of {tokenMax} used.
							</p>
						</div>
						<Button
							onclick={() => (tokenCreateOpen = true)}
							disabled={tokens.length >= tokenMax}
						>
							Create Token
						</Button>
					</div>

					<Separator />

					{#if tokensLoading}
						<p class="text-sm text-muted-foreground">Loading tokens…</p>
					{:else if tokens.length === 0}
						<p class="text-sm text-muted-foreground">
							No tokens yet. Create one to authenticate scripts against the Onyx API.
						</p>
					{:else}
						<div class="flex flex-col gap-3">
							{#each tokens as tok (tok.id)}
								<div class="rounded-md border p-3">
									<div class="flex items-start justify-between gap-3">
										<div class="min-w-0 flex-1 space-y-1">
											<div class="flex items-center gap-2 text-sm font-medium">
												<span class="truncate">{tok.name}</span>
												<span class="rounded bg-muted px-2 py-0.5 text-xs font-normal text-muted-foreground">
													{scopeBadgeLabel(tok.scope)}
												</span>
											</div>
											<p class="font-mono text-xs text-muted-foreground">
												onyx_…{tok.tokenLast8}
											</p>
											<div class="grid grid-cols-3 gap-2 text-xs text-muted-foreground">
												<span>Created {formatTokenDate(tok.createdAt)}</span>
												<span>Last used {formatLastUsed(tok.lastUsedAt)}</span>
												<span>Expires {formatTokenDate(tok.expiresAt)}</span>
											</div>
										</div>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => askRevokeToken(tok)}
											class="shrink-0"
										>
											<Trash2 class="size-4" />
										</Button>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</TabsContent>
		</Tabs>
	{/if}
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
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={confirmDisableSharing}>
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
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={confirmDisableVersioning}>
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
			<AlertDialog.Action onclick={confirmRevokeToken}>
				Revoke
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<TokenCreateDialog bind:open={tokenCreateOpen} onCreated={handleTokenCreated} />
