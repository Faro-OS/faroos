<script lang="ts">
	import { onDestroy } from 'svelte';
	import { getSection, setSection, type SectionId } from '$lib/section.svelte';
	import appsIcon from '$lib/assets/dock/apps.png';
	import containersIcon from '$lib/assets/dock/containers.png';
	import filesIcon from '$lib/assets/dock/files.png';
	import homeIcon from '$lib/assets/dock/home.png';
	import serversIcon from '$lib/assets/dock/servers.png';
	import settingsIcon from '$lib/assets/dock/settings.png';
	import terminalIcon from '$lib/assets/dock/terminal.png';
	import trashIcon from '$lib/assets/dock/trash.png';

	type DockItem = { id: SectionId | 'trash'; label: string; icon?: string };
	const items: DockItem[] = [
		{ id: 'dashboard', label: 'Home', icon: homeIcon },
		{ id: 'files', label: 'Files', icon: filesIcon },
		{ id: 'servers', label: 'Servers', icon: serversIcon },
		{ id: 'containers', label: 'Docker', icon: containersIcon },
		{ id: 'storage', label: 'Storage' },
		{ id: 'apps', label: 'App Store', icon: appsIcon },
		{ id: 'terminal', label: 'Terminal', icon: terminalIcon },
		{ id: 'settings', label: 'Settings', icon: settingsIcon },
		{ id: 'trash', label: 'Trash', icon: trashIcon }
	];

	const MAX_SCALE = 1.2;
	const RADIUS = 100;
	let buttons: (HTMLButtonElement | null)[] = $state([]);
	let scales = $state(items.map(() => 1));
	let hovered = $state<number | null>(null);
	let settleFrame: number | null = null;
	let settleTime = 0;
	let velocities = items.map(() => 0);

	function stopSettle() {
		if (settleFrame !== null) cancelAnimationFrame(settleFrame);
		settleFrame = null;
	}

	function handlePointerMove(event: PointerEvent) {
		if (window.innerWidth < 640 || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		stopSettle();
		velocities = items.map(() => 0);
		scales = buttons.map((button) => {
			if (!button) return 1;
			const rect = button.getBoundingClientRect();
			const distance = Math.abs(event.clientX - (rect.left + rect.width / 2));
			return distance >= RADIUS ? 1 : 1 + (1 - distance / RADIUS) * (MAX_SCALE - 1);
		});
	}

	function reset() {
		hovered = null;
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
			stopSettle();
			scales = items.map(() => 1);
			return;
		}
		stopSettle();
		settleTime = performance.now();
		settleFrame = requestAnimationFrame(settleToRest);
	}

	// Critically damped spring: start from the live value, stay interruptible
	// and settle without decorative bounce when the pointer leaves the dock.
	function settleToRest(now: number) {
		const dt = Math.min((now - settleTime) / 1000, 1 / 30);
		settleTime = now;
		let moving = false;
		const nextScales = scales.map((current, index) => {
			const acceleration = 170 * (1 - current) - 26 * velocities[index];
			velocities[index] += acceleration * dt;
			const next = current + velocities[index] * dt;
			if (Math.abs(next - 1) > 0.001 || Math.abs(velocities[index]) > 0.005) moving = true;
			return next;
		});
		scales = moving ? nextScales : items.map(() => 1);
		if (moving) settleFrame = requestAnimationFrame(settleToRest);
		else settleFrame = null;
	}

	function activate(item: DockItem) {
		setSection(item.id === 'trash' ? 'files' : item.id);
	}

	onDestroy(stopSettle);
</script>

<nav class="apple-dock" aria-label="FaroOS navigation" onpointermove={handlePointerMove} onpointerleave={reset}>
	{#each items as item, index (item.id)}
		{#if index === 7 || index === 8}<span class="dock-separator" aria-hidden="true"></span>{/if}
		<div class="dock-item">
			{#if hovered === index}<span class="dock-tooltip">{item.label}</span>{/if}
			<button
				bind:this={buttons[index]}
				type="button"
				aria-label={item.label}
				aria-current={item.id !== 'trash' && getSection() === item.id ? 'page' : undefined}
				onmouseenter={() => (hovered = index)}
				onmouseleave={() => (hovered = null)}
				onclick={() => activate(item)}
				style={`transform:scale(${scales[index]}) translateY(${(scales[index] - 1) * -16}px)`}
				class:active={item.id !== 'trash' && getSection() === item.id}
				class="dock-button"
			>
				{#if item.icon}
					<img src={item.icon} alt="" draggable="false" />
				{:else}
					<svg class="storage-icon" viewBox="0 0 34 34"><defs><linearGradient id="disk-metal" x1="0" y1="0" x2="0" y2="1"><stop stop-color="#f2f3f4"/><stop offset="1" stop-color="#8d939b"/></linearGradient></defs><rect x="4" y="4" width="26" height="10" rx="4"/><rect x="4" y="19" width="26" height="10" rx="4"/><circle cx="25" cy="9" r="1.5"/><circle cx="25" cy="24" r="1.5"/></svg>
				{/if}
			{#if item.id !== 'trash' && getSection() === item.id}<i class="active-dot"></i>{/if}
			</button>
		</div>
	{/each}
</nav>

<style>
	.apple-dock { position: fixed; z-index: 50; bottom: max(16px, env(safe-area-inset-bottom)); left: 50%; width: min(742px, calc(100vw - 24px)); height: 88px; display: flex; align-items: center; justify-content: center; gap: 13px; padding: 11px 20px 10px; border: 1px solid var(--border-strong); border-radius: 32px; background: color-mix(in srgb, var(--chrome) 94%, transparent); box-shadow: inset 0 1px 0 color-mix(in srgb, white 42%, transparent), 0 28px 74px rgba(14,35,68,.22), 0 4px 14px rgba(12,18,28,.08); backdrop-filter: blur(40px) saturate(1.8); -webkit-backdrop-filter: blur(40px) saturate(1.8); transform: translateX(-50%); }
	.dock-item { position: relative; display: flex; align-items: center; justify-content: center; }
	.dock-button { position: relative; width: 52px; height: 52px; display: grid; flex: 0 0 auto; place-items: center; border: 0; border-radius: 15px; background: transparent; transform-origin: center bottom; transition: filter 180ms ease, scale 90ms ease; will-change: transform; }
	.dock-button:hover { filter: brightness(1.04) saturate(1.04); }.dock-button:active { transition-duration: 75ms; }
	.dock-button.active img,.dock-button.active .storage-icon { filter: drop-shadow(0 7px 10px color-mix(in srgb,var(--accent) 20%,rgba(20,25,35,.12))) brightness(1.035); }
	.dock-button img { width: 52px; height: 52px; border-radius: 15px; object-fit: cover; user-select: none; -webkit-user-drag: none; filter: drop-shadow(0 5px 7px rgba(20,25,35,.13)); }
	.dock-button .storage-icon { width: 46px; height: 46px; padding: 5px; overflow: visible; border-radius: 14px; background: color-mix(in srgb, var(--surface-solid) 92%, transparent); filter: drop-shadow(0 5px 6px rgba(20,25,35,.14)); }.dock-button .storage-icon rect { fill: url(#disk-metal); stroke: #737a83; stroke-width: 1; }.dock-button .storage-icon circle { fill: #30d158; }
	.dock-separator { width: 1px; height: 42px; flex: 0 0 auto; margin: 0 -4px; background: var(--border-strong); }
	.dock-tooltip { position: absolute; bottom: 66px; left: 50%; padding: 6px 10px; border: 1px solid var(--border); border-radius: 9px; color: var(--fg); background: color-mix(in srgb, var(--surface-raised) 94%, transparent); box-shadow: var(--shadow-md); font-size: 10px; font-weight: 600; white-space: nowrap; transform: translateX(-50%); transform-origin: center bottom; animation: tooltip-in 150ms var(--motion-settle) both; backdrop-filter: blur(20px) saturate(1.4); }
	.active-dot { position: absolute; bottom: -8px; left: 50%; width: 5px; height: 5px; border-radius: 50%; background: var(--accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 9%, transparent),0 0 10px color-mix(in srgb,var(--accent) 42%,transparent); transform: translateX(-50%); }
	@keyframes tooltip-in { from { opacity: 0; transform: translateX(-50%) translateY(4px) scale(.96); } to { opacity: 1; transform: translateX(-50%) translateY(0) scale(1); } }
	@media (max-width: 760px) { .apple-dock { width: min(680px,calc(100vw - 16px)); height: 74px; gap: 7px; padding: 9px 11px; border-radius: 28px; }.dock-button,.dock-button img { width: 43px; height: 43px; border-radius: 13px; }.dock-button .storage-icon { width: 41px; height: 41px; }.dock-separator { height: 36px; }.active-dot { bottom: -7px; } }
	@media (max-width: 540px) { .apple-dock { height: 66px; justify-content: space-around; gap: 1px; padding-inline: 6px; border-radius: 24px; }.dock-button,.dock-button img { width: 35px; height: 35px; border-radius: 10px; }.dock-button .storage-icon { width: 34px; height: 34px; padding: 4px; border-radius: 10px; }.dock-separator { height: 30px; margin: 0 -1px; }.dock-tooltip { display:none; }.active-dot { bottom: -7px; width: 3px; height: 3px; } }
	@media (prefers-reduced-motion: reduce) { .dock-button { transform: none !important; }.dock-tooltip { animation: none; } }
	@media (prefers-reduced-transparency: reduce) { .apple-dock { background: var(--surface-solid); backdrop-filter: none; -webkit-backdrop-filter: none; } }
	@media (prefers-contrast: more) { .apple-dock { border-color: var(--border-strong); background: var(--surface-solid); }.active-dot { background: var(--accent); } }
</style>
