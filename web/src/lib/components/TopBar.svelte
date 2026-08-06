<script lang="ts">
	let { title, children }: { title: string; children?: import('svelte').Snippet } = $props();
</script>

<header class="flex items-center justify-between gap-3 border-b border-[var(--border)] bg-[var(--bg)] px-4 py-4 sm:px-6">
	<h1 class="min-w-0 truncate text-xl font-semibold text-[var(--fg)]">{title}</h1>
	<div class="flex shrink-0 items-center gap-3">
		{@render children?.()}
	</div>
</header>
