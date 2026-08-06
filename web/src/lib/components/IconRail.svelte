<script lang="ts">
	import { getSection, setSection, type SectionId } from '$lib/section.svelte';
	import { getTheme, toggleTheme } from '$lib/theme.svelte';

	const items: { id: SectionId; label: string; icon: string }[] = [
		{ id: 'dashboard', label: 'Home', icon: 'M4 13h6V4H4v9Zm0 7h6v-5H4v5Zm10 0h6V11h-6v9Zm0-16v5h6V4h-6Z' },
		{ id: 'servers', label: 'Servers', icon: 'M4 6h16M4 12h16M4 18h16' },
		{ id: 'containers', label: 'Containers', icon: 'M21 8 12 3 3 8m18 0-9 5m9-5v9l-9 5m0-9L3 8m9 5v9M3 8v9l9 5' },
		{ id: 'storage', label: 'Storage', icon: 'M4 7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7Zm4 3h.01M8 14h.01' },
		{ id: 'apps', label: 'App Store', icon: 'M12 3v18m9-9H3' },
		{ id: 'terminal', label: 'Terminal', icon: 'm4 6 5 6-5 6m8 0h8' },
		{ id: 'files', label: 'Files', icon: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z' },
		{ id: 'settings', label: 'Settings', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7-3c0 .34-.02.67-.06 1l2.11 1.65-2 3.46-2.49-1a7.4 7.4 0 0 1-1.73 1L14.5 21h-5l-.33-2.89a7.4 7.4 0 0 1-1.73-1l-2.49 1-2-3.46L4.06 13A7.6 7.6 0 0 1 4 12c0-.34.02-.67.06-1L1.95 9.35l2-3.46 2.49 1c.53-.42 1.11-.76 1.73-1L8.5 3h5l.33 2.89c.62.24 1.2.58 1.73 1l2.49-1 2 3.46L17.94 11c.04.33.06.66.06 1Z' }
	];
</script>

<nav
	class="fixed left-4 top-4 z-40 flex items-center gap-1 rounded-2xl border border-white/10 bg-black/30 p-1.5 shadow-lg backdrop-blur-md"
>
	{#each items as item (item.id)}
		<button
			type="button"
			title={item.label}
			aria-label={item.label}
			onclick={() => setSection(item.id)}
			class="grid h-9 w-9 place-items-center rounded-xl transition-colors
				{getSection() === item.id ? 'bg-white/90 text-black' : 'text-white/80 hover:bg-white/15 hover:text-white'}"
		>
			<svg viewBox="0 0 24 24" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="2">
				<path d={item.icon} stroke-linecap="round" stroke-linejoin="round" />
			</svg>
		</button>
	{/each}
	<div class="mx-1 h-5 w-px bg-white/15"></div>
	<button
		type="button"
		title="Toggle theme"
		aria-label="Toggle theme"
		onclick={toggleTheme}
		class="grid h-9 w-9 place-items-center rounded-xl text-white/80 transition-colors hover:bg-white/15 hover:text-white"
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
</nav>
