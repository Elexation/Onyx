<script lang="ts">
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";
	import { getSettings, updateSettings, changePassword } from "$lib/api/settings";
	import { Tabs, TabsList, TabsTrigger, TabsContent } from "$lib/components/ui/tabs/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Separator } from "$lib/components/ui/separator/index.js";

	const MIN_PASSWORD_LENGTH = 8;

	const caps: Record<string, { min: number; max: number; label: string }> = {
		"versions.max_count": { min: 0, max: 10000, label: "Max versions" },
		"versions.max_age": { min: 0, max: 87600, label: "Max version age" },
		"versions.max_file_size": { min: 0, max: 102400, label: "Max file size to version" },
		"versions.max_storage": { min: 0, max: 102400, label: "Max version storage" },
		"trash.purge_age": { min: 0, max: 87600, label: "Trash purge age" },
		"trash.max_size": { min: 0, max: 102400, label: "Max trash size" },
		"shares.default_expiry": { min: 0, max: 87600, label: "Share expiry" },
		"session.lifetime": { min: 1, max: 87600, label: "Session lifetime" },
		"upload.max_size": { min: 0, max: 102400, label: "Max file size" },
	};

	let settings = $state<Record<string, string>>({});
	let loading = $state(true);

	let currentPassword = $state("");
	let newPassword = $state("");
	let confirmPassword = $state("");
	let changingPassword = $state(false);

	let debounceTimers: Record<string, ReturnType<typeof setTimeout>> = {};

	onMount(async () => {
		try {
			settings = await getSettings();
		} catch {
			toast.error("Failed to load settings");
		} finally {
			loading = false;
		}
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
		<Tabs value="general">
			<TabsList class="mb-6">
				<TabsTrigger value="general">General</TabsTrigger>
				<TabsTrigger value="versioning">Versioning</TabsTrigger>
				<TabsTrigger value="trash">Trash</TabsTrigger>
				<TabsTrigger value="sharing">Sharing</TabsTrigger>
				<TabsTrigger value="uploads">Uploads</TabsTrigger>
				<TabsTrigger value="security">Security</TabsTrigger>
			</TabsList>

			<!-- General -->
			<TabsContent value="general">
				<div class="space-y-4">
					<div class="space-y-2">
						<Label for="branding-name">Application name</Label>
						<Input
							id="branding-name"
							value={settings["branding.name"] ?? ""}
							onchange={(e) => save("branding.name", e.currentTarget.value)}
							class="max-w-xs"
						/>
					</div>
				</div>
			</TabsContent>

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
							onCheckedChange={(checked: boolean) => toggleBool("versions.enabled", checked)}
						/>
					</div>
					<Separator />
					<div class="space-y-2">
						<Label for="versions-max-count">Maximum versions per file</Label>
						<Input
							id="versions-max-count"
							type="number"
							min="0"
							max="10000"
							step="1"
							value={settings["versions.max_count"] ?? "10"}
							onchange={(e) => validateAndSaveInt("versions.max_count", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 10,000</p>
					</div>
					<div class="space-y-2">
						<Label for="versions-max-age">Maximum version age (hours)</Label>
						<Input
							id="versions-max-age"
							type="number"
							min="0"
							max="87600"
							step="1"
							value={durationToHours(settings["versions.max_age"] ?? "2160h")}
							onchange={(e) => validateAndSaveDuration("versions.max_age", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = never expire. Max: 87,600 hours (10 years). Default: 2,160 (90 days)</p>
					</div>
					<div class="space-y-2">
						<Label for="versions-max-file-size">Maximum file size to version (MB)</Label>
						<Input
							id="versions-max-file-size"
							type="number"
							min="0"
							max="102400"
							step="1"
							value={bytesToMB(settings["versions.max_file_size"] ?? "1073741824")}
							onchange={(e) => validateAndSaveMB("versions.max_file_size", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 102,400 MB (100 GB). Default: 1,024 (1 GB). Files larger than this are not versioned.</p>
					</div>
					<div class="space-y-2">
						<Label for="versions-max-storage">Maximum version storage (MB)</Label>
						<Input
							id="versions-max-storage"
							type="number"
							min="0"
							max="102400"
							step="1"
							value={bytesToMB(settings["versions.max_storage"] ?? "0")}
							onchange={(e) => validateAndSaveMB("versions.max_storage", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = unlimited. Max: 102,400 MB (100 GB). Oldest versions are purged when exceeded.</p>
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
							max="87600"
							step="1"
							value={durationToHours(settings["trash.purge_age"] ?? "720h")}
							onchange={(e) => validateAndSaveDuration("trash.purge_age", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = never purge. Max: 87,600 hours (10 years). Default: 720 (30 days)</p>
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
							onCheckedChange={(checked: boolean) => toggleBool("shares.enabled", checked)}
						/>
					</div>
					<Separator />
					<div class="space-y-2">
						<Label for="shares-default-expiry">Default share expiry (hours)</Label>
						<Input
							id="shares-default-expiry"
							type="number"
							min="0"
							max="87600"
							step="1"
							value={durationToHours(settings["shares.default_expiry"] ?? "168h")}
							onchange={(e) => validateAndSaveDuration("shares.default_expiry", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">0 = never expire. Max: 87,600 hours (10 years). Default: 168 (7 days)</p>
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

			<!-- Security -->
			<TabsContent value="security">
				<div class="space-y-6">
					<div class="space-y-2">
						<Label for="session-lifetime">Session lifetime (hours)</Label>
						<Input
							id="session-lifetime"
							type="number"
							min="1"
							max="87600"
							step="1"
							value={durationToHours(settings["session.lifetime"] ?? "720h")}
							onchange={(e) => validateAndSaveDuration("session.lifetime", e.currentTarget.value)}
							class="max-w-xs"
						/>
						<p class="text-xs text-muted-foreground">1–87,600 hours (10 years). Default: 720 (30 days). Only affects new sessions.</p>
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
		</Tabs>
	{/if}
</div>
