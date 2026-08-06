<script lang="ts">
	import { getTheme, toggleTheme } from '$lib/theme.svelte';
	import { toggleSidebar } from '$lib/sidebar.svelte';

	let { title, children }: { title: string; children?: import('svelte').Snippet } = $props();
</script>

<header class="flex items-center justify-between gap-3 border-b border-[var(--border)] bg-[var(--bg)] px-4 py-4 sm:px-6">
	<div class="flex min-w-0 items-center gap-3">
		<button
			onclick={toggleSidebar}
			aria-label="Toggle menu"
			class="grid h-9 w-9 shrink-0 place-items-center rounded-full border border-[var(--border)] bg-[var(--surface)] text-[var(--fg-muted)] hover:text-[var(--fg)] md:hidden"
		>
			<svg viewBox="0 0 24 24" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M4 6h16M4 12h16M4 18h16" stroke-linecap="round" />
			</svg>
		</button>
		<h1 class="truncate text-xl font-semibold text-[var(--fg)]">{title}</h1>
	</div>
	<div class="flex shrink-0 items-center gap-3">
		{@render children?.()}
		<button
			onclick={toggleTheme}
			aria-label="Toggle theme"
			class="grid h-9 w-9 place-items-center rounded-full border border-[var(--border)] bg-[var(--surface)] text-[var(--fg-muted)] transition-colors hover:text-[var(--fg)]"
		>
			{#if getTheme() === 'dark'}
				<svg viewBox="0 0 24 24" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="4" />
					<path d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41m11.32-11.32 1.41-1.41" stroke-linecap="round" />
				</svg>
			{:else}
				<svg viewBox="0 0 24 24" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" stroke-linecap="round" stroke-linejoin="round" />
				</svg>
			{/if}
		</button>
	</div>
</header>
