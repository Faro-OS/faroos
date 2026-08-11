<script lang="ts">
	let {
		title,
		subtitle,
		children
	}: { title: string; subtitle?: string; children?: import('svelte').Snippet } = $props();

	const descriptions: Record<string, string> = {
		Servers: 'Your connected infrastructure',
		Containers: 'Services and workloads',
		Storage: 'Disks, volumes and capacity',
		'App Store': 'Discover your next service',
		Terminal: 'Secure command line access',
		Files: 'Browse every connected server',
		Settings: 'Make FaroOS yours'
	};

	function openSpotlight() {
		window.dispatchEvent(new CustomEvent('faroos:spotlight'));
	}
</script>

<header class="platform-topbar">
	<div class="topbar-inner mx-auto flex min-h-[72px] max-w-[1480px] items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
		<div class="min-w-0 py-3">
			<p class="eyebrow mb-1.5">FaroOS · Control center</p>
			<div class="flex min-w-0 items-baseline gap-3">
				<h1 class="truncate text-[1.55rem] font-semibold leading-none tracking-[-0.035em] text-[var(--fg)]">{title}</h1>
				<p class="hidden truncate text-sm text-[var(--fg-subtle)] md:block">{subtitle ?? descriptions[title] ?? ''}</p>
			</div>
		</div>

		<div class="flex max-w-[66vw] shrink-0 items-center gap-2 overflow-x-auto py-1">
			<div class="flex items-center gap-2">
				{@render children?.()}
			</div>
			<button type="button" onclick={openSpotlight} class="control hidden h-10 items-center gap-2 rounded-xl px-3 text-sm text-[var(--fg-muted)] sm:flex" aria-label="Search FaroOS">
				<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3-3" stroke-linecap="round" /></svg>
				<span class="hidden sm:inline">Search</span>
				<kbd class="hidden rounded-md border border-[var(--border)] bg-[var(--track)] px-1.5 py-0.5 text-[10px] text-[var(--fg-subtle)] sm:inline">⌘K</kbd>
			</button>
		</div>
	</div>
</header>

<style>
	.platform-topbar {
		position: sticky;
		z-index: 30;
		top: 12px;
		width: min(1480px, calc(100% - 24px));
		margin: 12px auto 0;
		border: 1px solid var(--border-strong);
		border-radius: 24px;
		background: color-mix(in srgb, var(--chrome) 92%, transparent);
		box-shadow: inset 0 1px 0 color-mix(in srgb, white 30%, transparent), var(--shadow-md);
		backdrop-filter: blur(36px) saturate(1.65);
		-webkit-backdrop-filter: blur(36px) saturate(1.65);
	}

	.platform-topbar::after {
		position: absolute;
		right: 0;
		bottom: -18px;
		left: 0;
		height: 14px;
		content: '';
		background: linear-gradient(to bottom, color-mix(in srgb, var(--fg) 7%, transparent), transparent 45%);
		mask-image: linear-gradient(to bottom, black, transparent);
		pointer-events: none;
		opacity: 0.18;
	}

	.topbar-inner {
		position: relative;
	}

	@media (max-width: 640px) {
		.platform-topbar {
			top: 8px;
			width: calc(100% - 16px);
			margin-top: 8px;
			border-radius: 20px;
			background: color-mix(in srgb, var(--chrome) 94%, transparent);
		}
	}

	@media (prefers-reduced-transparency: reduce) {
		.platform-topbar {
			background: var(--bg-elevated);
			backdrop-filter: none;
			-webkit-backdrop-filter: none;
		}
	}

	@media (prefers-contrast: more) {
		.platform-topbar {
			border-bottom: 1px solid var(--border-strong);
			background: var(--surface-solid);
		}
	}
</style>
