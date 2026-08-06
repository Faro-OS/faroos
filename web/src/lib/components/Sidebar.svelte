<script lang="ts">
	import { page } from '$app/state';

	const items = [
		{ href: '/', label: 'Dashboard', icon: 'M4 13h6V4H4v9Zm0 7h6v-5H4v5Zm10 0h6V11h-6v9Zm0-16v5h6V4h-6Z' },
		{ href: '/nodes', label: 'Servers', icon: 'M4 6h16M4 12h16M4 18h16' },
		{ href: '/containers', label: 'Containers', icon: 'M21 8 12 3 3 8m18 0-9 5m9-5v9l-9 5m0-9L3 8m9 5v9M3 8v9l9 5' },
		{ href: '/storage', label: 'Storage', icon: 'M4 7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7Zm4 3h.01M8 14h.01' },
		{ href: '/apps', label: 'App Store', icon: 'M12 3v18m9-9H3' },
		{ href: '/terminal', label: 'Terminal', icon: 'm4 6 5 6-5 6m8 0h8' },
		{ href: '/files', label: 'Files', icon: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z' },
		{ href: '/settings', label: 'Settings', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7-3c0 .34-.02.67-.06 1l2.11 1.65-2 3.46-2.49-1a7.4 7.4 0 0 1-1.73 1L14.5 21h-5l-.33-2.89a7.4 7.4 0 0 1-1.73-1l-2.49 1-2-3.46L4.06 13A7.6 7.6 0 0 1 4 12c0-.34.02-.67.06-1L1.95 9.35l2-3.46 2.49 1c.53-.42 1.11-.76 1.73-1L8.5 3h5l.33 2.89c.62.24 1.2.58 1.73 1l2.49-1 2 3.46L17.94 11c.04.33.06.66.06 1Z' }
	];
</script>

<aside class="flex w-56 shrink-0 flex-col gap-1 border-r border-[var(--border)] bg-[var(--sidebar-bg)] p-3">
	<div class="mb-4 flex items-center gap-2 px-2 pt-1">
		<span class="grid h-8 w-8 place-items-center rounded-xl bg-[var(--accent)] text-[var(--accent-fg)]">
			<svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M12 2 3 7v6c0 5 4 8.5 9 9 5-.5 9-4 9-9V7l-9-5Z" stroke-linecap="round" stroke-linejoin="round" />
				<path d="M12 8v5l3 2" stroke-linecap="round" stroke-linejoin="round" />
			</svg>
		</span>
		<span class="text-lg font-semibold tracking-tight text-[var(--fg)]">FaroOS</span>
	</div>

	{#each items as item (item.href)}
		<a
			href={item.href}
			class="flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition-colors
				{page.url.pathname === item.href
				? 'bg-[var(--accent)]/12 text-[var(--accent)]'
				: 'text-[var(--fg-muted)] hover:bg-[var(--surface-raised)] hover:text-[var(--fg)]'}"
		>
			<svg viewBox="0 0 24 24" class="h-[18px] w-[18px] shrink-0" fill="none" stroke="currentColor" stroke-width="2">
				<path d={item.icon} stroke-linecap="round" stroke-linejoin="round" />
			</svg>
			{item.label}
		</a>
	{/each}
</aside>
