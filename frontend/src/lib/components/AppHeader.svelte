<script lang="ts">
	import { logout } from "$lib/auth.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { LogOut } from "lucide-svelte";
	import SearchBar from "./SearchBar.svelte";
</script>

<header class="flex h-14 shrink-0 items-center border-b border-border bg-card px-4">
	<a href="/files" class="text-lg font-semibold tracking-tight">Onyx</a>

	<div class="mx-auto w-full max-w-md px-4">
		<SearchBar />
	</div>

	<div class="flex items-center gap-2">
		<Button variant="ghost" size="icon-sm" onclick={logout} title="Sign out">
			<LogOut class="size-4" />
		</Button>
	</div>
</header>
