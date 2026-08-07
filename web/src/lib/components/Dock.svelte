<script lang="ts">
	import { getSection, setSection, type SectionId } from '$lib/section.svelte';
	import { getTheme, toggleTheme } from '$lib/theme.svelte';
	import logo from '$lib/assets/logo.png';

	type Item = { id: SectionId; label: string; icon: string };

	const items: Item[] = [
		{ id: 'dashboard', label: 'Home', icon: 'M4 13h6V4H4v9Zm0 7h6v-5H4v5Zm10 0h6V11h-6v9Zm0-16v5h6V4h-6Z' },
		{ id: 'servers', label: 'Servers', icon: 'M4 6h16M4 12h16M4 18h16' },
		{ id: 'containers', label: 'Containers', icon: 'M21 8 12 3 3 8m18 0-9 5m9-5v9l-9 5m0-9L3 8m9 5v9M3 8v9l9 5' },
		{ id: 'storage', label: 'Storage', icon: 'M4 7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7Zm4 3h.01M8 14h.01' },
		{ id: 'apps', label: 'App Store', icon: 'M12 3v18m9-9H3' },
		{ id: 'terminal', label: 'Terminal', icon: 'm4 6 5 6-5 6m8 0h8' },
		{ id: 'files', label: 'Files', icon: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z' },
		{ id: 'settings', label: 'Settings', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7-3c0 .34-.02.67-.06 1l2.11 1.65-2 3.46-2.49-1a7.4 7.4 0 0 1-1.73 1L14.5 21h-5l-.33-2.89a7.4 7.4 0 0 1-1.73-1l-2.49 1-2-3.46L4.06 13A7.6 7.6 0 0 1 4 12c0-.34.02-.67.06-1L1.95 9.35l2-3.46 2.49 1c.53-.42 1.11-.76 1.73-1L8.5 3h5l.33 2.89c.62.24 1.2.58 1.73 1l2.49-1 2 3.46L17.94 11c.04.33.06.66.06 1Z' }
	];

	const MAX_SCALE = 1.55;
	const RADIUS = 110; // px of mouse influence on either side of an icon

	let buttons: (HTMLButtonElement | null)[] = $state([]);
	let scales = $state<number[]>(items.map(() => 1));
	let hovered = $state<number | null>(null);

	function handleMouseMove(e: MouseEvent) {
		scales = buttons.map((btn) => {
			if (!btn) return 1;
			const rect = btn.getBoundingClientRect();
			const center = rect.left + rect.width / 2;
			const dist = Math.abs(e.clientX - center);
			if (dist > RADIUS) return 1;
			return 1 + (1 - dist / RADIUS) * (MAX_SCALE - 1);
		});
	}

	function resetScales() {
		scales = items.map(() => 1);
		hovered = null;
	}
</script>

<nav
	aria-label="FaroOS"
	onmousemove={handleMouseMove}
	onmouseleave={resetScales}
	class="glass-dark fixed bottom-2 left-1/2 z-40 flex max-w-[calc(100vw-1rem)] -translate-x-1/2 items-end gap-1 overflow-x-auto rounded-[24px] px-2 py-1.5 sm:bottom-4 sm:max-w-[calc(100vw-2rem)] sm:gap-1.5 sm:rounded-[28px] sm:px-2.5 sm:py-2"
>
	<span class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-white p-1 shadow sm:h-11 sm:w-11 sm:rounded-2xl sm:p-1.5">
		<img src={logo} alt="FaroOS" class="h-full w-full object-contain" />
	</span>
	<div class="mx-0.5 h-6 w-px shrink-0 self-center bg-white/15 sm:mx-1 sm:h-8"></div>

	{#each items as item, i (item.id)}
		<div class="relative flex shrink-0 flex-col items-center">
			{#if hovered === i}
				<span
					class="glass-dark pointer-events-none absolute -top-10 hidden whitespace-nowrap rounded-xl px-2.5 py-1 text-xs font-medium text-white sm:block"
				>
					{item.label}
				</span>
			{/if}
			<button
				bind:this={buttons[i]}
				type="button"
				aria-label={item.label}
				onmouseenter={() => (hovered = i)}
				onmouseleave={() => (hovered = null)}
				onclick={() => setSection(item.id)}
				style="transform: scale({scales[i]}) translateY({(scales[i] - 1) * -14}px);"
				class="grid h-9 w-9 shrink-0 origin-bottom place-items-center rounded-xl transition-[background-color,color] duration-150 will-change-transform sm:h-11 sm:w-11 sm:rounded-2xl
					{getSection() === item.id ? 'bg-white text-black' : 'bg-white/10 text-white/85 hover:bg-white/20 hover:text-white'}"
			>
				<svg viewBox="0 0 24 24" class="h-4 w-4 sm:h-5 sm:w-5" fill="none" stroke="currentColor" stroke-width="2">
					<path d={item.icon} stroke-linecap="round" stroke-linejoin="round" />
				</svg>
			</button>
			<span class="mt-1 hidden h-1 w-1 rounded-full sm:block {getSection() === item.id ? 'bg-white' : 'bg-transparent'}"></span>
		</div>
	{/each}

	<div class="mx-0.5 h-6 w-px shrink-0 self-center bg-white/15 sm:mx-1 sm:h-8"></div>
	<button
		type="button"
		title="Toggle theme"
		aria-label="Toggle theme"
		onclick={toggleTheme}
		class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-white/10 text-white/85 transition-colors hover:bg-white/20 hover:text-white sm:h-11 sm:w-11 sm:rounded-2xl"
	>
		{#if getTheme() === 'dark'}
			<svg viewBox="0 0 24 24" class="h-4 w-4 sm:h-5 sm:w-5" fill="none" stroke="currentColor" stroke-width="2">
				<circle cx="12" cy="12" r="4" />
				<path d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41m11.32-11.32 1.41-1.41" stroke-linecap="round" />
			</svg>
		{:else}
			<svg viewBox="0 0 24 24" class="h-4 w-4 sm:h-5 sm:w-5" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" stroke-linecap="round" stroke-linejoin="round" />
			</svg>
		{/if}
	</button>
</nav>
