<script lang="ts">
	import { setSection, type SectionId } from '$lib/section.svelte';
	import { toggleTheme } from '$lib/theme.svelte';

	type SearchItem = {
		label: string;
		description: string;
		section?: SectionId;
		keywords: string;
		icon: string;
		action?: () => void;
	};

	const items: SearchItem[] = [
		{ label: 'Overview', description: 'Your entire system at a glance', section: 'dashboard', keywords: 'home dashboard overview status', icon: 'M4 13h6V4H4v9Zm0 7h6v-5H4v5Zm10 0h6V11h-6v9Zm0-16v5h6V4h-6Z' },
		{ label: 'Servers', description: 'Health, uptime and connected nodes', section: 'servers', keywords: 'nodes machines cpu memory ram', icon: 'M5 5h14v5H5V5Zm0 9h14v5H5v-5Zm3-6h.01M8 17h.01' },
		{ label: 'Containers', description: 'Start, stop and inspect workloads', section: 'containers', keywords: 'docker logs restart services', icon: 'M21 8 12 3 3 8m18 0-9 5m9-5v9l-9 5m0-9L3 8m9 5v9M3 8v9l9 5' },
		{ label: 'Storage', description: 'Volumes, disks and capacity', section: 'storage', keywords: 'drive disk volumes space', icon: 'M4 7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7Zm4 3h.01M8 14h.01' },
		{ label: 'App Store', description: 'Discover and deploy new services', section: 'apps', keywords: 'applications install catalog store', icon: 'M12 3v18m9-9H3' },
		{ label: 'Terminal', description: 'Open a secure shell session', section: 'terminal', keywords: 'shell console ssh command', icon: 'm4 6 5 6-5 6m8 0h8' },
		{ label: 'Files', description: 'Browse and manage server files', section: 'files', keywords: 'folders upload download browser', icon: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z' },
		{ label: 'Settings', description: 'Appearance, account and system', section: 'settings', keywords: 'preferences theme account about', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7-3a7 7 0 0 1-.2 1.7l1.8 1.4-2 3.4-2.2-.9a7 7 0 0 1-2.9 1.7L13.2 22h-4l-.4-2.7a7 7 0 0 1-2.9-1.7l-2.5.9-2-3.4 2.1-1.4a7 7 0 0 1 0-3.4L1.4 8.9l2-3.4 2.5.9a7 7 0 0 1 2.9-1.7L9.2 2h4l.4 2.7a7 7 0 0 1 2.9 1.7l2.2-.9 2 3.4-1.8 1.4c.1.5.2 1.1.2 1.7Z' },
		{ label: 'Switch appearance', description: 'Toggle light or dark mode', keywords: 'dark light theme appearance', icon: 'M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8Z', action: toggleTheme }
	];

	let open = $state(false);
	let query = $state('');
	let activeIndex = $state(0);
	let input = $state<HTMLInputElement>();

	const filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return items;
		return items.filter((item) => `${item.label} ${item.description} ${item.keywords}`.toLowerCase().includes(q));
	});

	function show() {
		open = true;
		query = '';
		activeIndex = 0;
		setTimeout(() => input?.focus(), 0);
	}

	function hide() {
		open = false;
	}

	function choose(item: SearchItem) {
		if (item.section) setSection(item.section);
		item.action?.();
		hide();
	}

	function handleKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			open ? hide() : show();
			return;
		}
		if (!open) return;
		if (e.key === 'Escape') hide();
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			activeIndex = Math.min(filtered.length - 1, activeIndex + 1);
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			activeIndex = Math.max(0, activeIndex - 1);
		}
		if (e.key === 'Enter' && filtered[activeIndex]) choose(filtered[activeIndex]);
	}

	$effect(() => {
		const handler = () => show();
		window.addEventListener('faroos:spotlight', handler);
		return () => window.removeEventListener('faroos:spotlight', handler);
	});
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div class="spotlight-layer fixed inset-0 z-[80] flex justify-center px-3 pt-[12vh]">
		<button type="button" aria-label="Close search" onclick={hide} class="spotlight-scrim absolute inset-0 cursor-default"></button>
		<div class="spotlight-panel glass relative h-fit w-full max-w-xl overflow-hidden rounded-[28px]" role="dialog" aria-modal="true" aria-label="Search FaroOS" tabindex="-1">
			<div class="flex items-center gap-3 border-b border-[var(--border)] px-5">
				<svg viewBox="0 0 24 24" class="h-5 w-5 shrink-0 text-[var(--fg-subtle)]" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="11" cy="11" r="7" />
					<path d="m20 20-3-3" stroke-linecap="round" />
				</svg>
				<input bind:this={input} bind:value={query} oninput={() => (activeIndex = 0)} placeholder="Search settings, apps and actions…" class="h-16 min-w-0 flex-1 bg-transparent text-base text-[var(--fg)] outline-none placeholder:text-[var(--fg-subtle)]" />
				<kbd class="rounded-md border border-[var(--border)] bg-[var(--track)] px-1.5 py-1 text-[10px] font-medium text-[var(--fg-subtle)]">ESC</kbd>
			</div>

			<div class="max-h-[52vh] overflow-y-auto p-2.5">
				{#if filtered.length === 0}
					<div class="grid place-items-center px-5 py-12 text-sm text-[var(--fg-subtle)]">No results for “{query}”</div>
				{:else}
					<p class="eyebrow px-3 pb-2 pt-2">Quick access</p>
					{#each filtered as item, i (item.label)}
						<button type="button" onmouseenter={() => (activeIndex = i)} onclick={() => choose(item)} class="flex w-full items-center gap-3 rounded-2xl px-3 py-2.5 text-left transition-colors {i === activeIndex ? 'bg-[var(--accent-soft)]' : 'hover:bg-[var(--track)]'}">
							<span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-[var(--border)] {i === activeIndex ? 'bg-[var(--accent)] text-white' : 'bg-[var(--surface-raised)] text-[var(--fg-muted)]'}">
								<svg viewBox="0 0 24 24" class="h-4.5 w-4.5" fill="none" stroke="currentColor" stroke-width="1.8"><path d={item.icon} stroke-linecap="round" stroke-linejoin="round" /></svg>
							</span>
							<span class="min-w-0 flex-1">
								<span class="block text-sm font-semibold text-[var(--fg)]">{item.label}</span>
								<span class="block truncate text-xs text-[var(--fg-subtle)]">{item.description}</span>
							</span>
							{#if i === activeIndex}<span class="text-xs text-[var(--fg-subtle)]">↵</span>{/if}
						</button>
					{/each}
				{/if}
			</div>
			<div class="flex items-center justify-between border-t border-[var(--border)] px-5 py-3 text-[11px] text-[var(--fg-subtle)]">
				<span>Navigate with ↑ ↓</span><span>FaroOS Control Center</span>
			</div>
		</div>
	</div>
{/if}

<style>
	.spotlight-layer {
		perspective: 1000px;
	}

	.spotlight-scrim {
		border: 0;
		background: color-mix(in srgb, #06070a 38%, transparent);
		animation: scrim-in 180ms ease-out both;
		backdrop-filter: blur(8px) saturate(0.85);
		-webkit-backdrop-filter: blur(8px) saturate(0.85);
	}

	.spotlight-panel {
		transform-origin: center 12%;
		animation: spotlight-in 340ms var(--motion-settle) both;
	}

	@keyframes scrim-in {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	@keyframes spotlight-in {
		from { opacity: 0; transform: translateY(-12px) scale(.965); filter: blur(8px); }
		to { opacity: 1; transform: translateY(0) scale(1); filter: blur(0); }
	}

	@media (prefers-reduced-motion: reduce) {
		.spotlight-scrim,
		.spotlight-panel { animation: none; }
	}

	@media (prefers-reduced-transparency: reduce) {
		.spotlight-scrim { backdrop-filter: none; -webkit-backdrop-filter: none; }
	}
</style>
