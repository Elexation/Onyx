<script lang="ts">
	import { logout } from "$lib/auth.svelte.js";
	import { Button } from "$lib/components/ui/button/index.js";
</script>

<div class="flex h-screen flex-col items-center justify-center gap-4">
	<h1 class="text-4xl font-bold tracking-tight">Onyx</h1>
	<p class="text-muted-foreground">File browser coming soon.</p>
	<Button variant="outline" onclick={logout}>Sign Out</Button>
</div>
