<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import AppIcon from '$lib/components/AppIcon.svelte';
	import InstallAppModal from '$lib/components/InstallAppModal.svelte';
	import {
		listApps,
		listAppCategories,
		listContainers,
		listNodes,
		refreshAppCatalog,
		removeApp,
		type CatalogApp,
		type Container,
		type Node
	} from '$lib/api';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	const PAGE_SIZE = 60;

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let apps = $state<CatalogApp[]>([]);
	let categories = $state<string[]>([]);
	let containers = $state<Container[]>([]);
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<string | null>(null);
	let busyAppId = $state<string | null>(null);
	let installTarget = $state<CatalogApp | null>(null);

	let search = $state('');
	let activeCategory = $state('All');
	let visibleCount = $state(PAGE_SIZE);

	const connectedNodes = $derived(nodes.filter((n) => n.connected));

	const filteredApps = $derived.by(() => {
		const q = search.trim().toLowerCase();
		return apps.filter((app) => {
			if (activeCategory !== 'All' && (app.category ?? 'Other') !== activeCategory) return false;
			if (!q) return true;
			return app.name.toLowerCase().includes(q) || app.description.toLowerCase().includes(q);
		});
	});

	const visibleApps = $derived(filteredApps.slice(0, visibleCount));

	$effect(() => {
		// Reset pagination whenever the filter changes.
		search;
		activeCategory;
		visibleCount = PAGE_SIZE;
	});

	function containerNameFor(appId: string): string {
		return `faroos-app-${appId}`;
	}

	function statusFor(appId: string): 'not-installed' | 'running' | 'stopped' {
		const target = '/' + containerNameFor(appId);
		const match = containers.find((c) => c.names.includes(target));
		if (!match) return 'not-installed';
		return match.state === 'running' ? 'running' : 'stopped';
	}

	async function loadNodes() {
		nodes = await listNodes();
		if (!selectedNodeId && connectedNodes.length > 0) {
			selectedNodeId = connectedNodes[0].id;
		}
	}

	async function loadContainers() {
		if (!selectedNodeId) {
			containers = [];
			return;
		}
		try {
			containers = await listContainers(selectedNodeId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load app status';
		}
	}

	$effect(() => {
		(async () => {
			loading = true;
			apps = await listApps().catch(() => []);
			categories = await listAppCategories().catch(() => []);
			await loadNodes();
			await loadContainers();
			loading = false;
		})();
	});

	$effect(() => {
		if (selectedNodeId) {
			void loadContainers();
		}
	});

	async function handleRefreshCatalog() {
		refreshing = true;
		try {
			const res = await refreshAppCatalog();
			apps = await listApps();
			categories = await listAppCategories().catch(() => []);
			toastSuccess(`Catalog refreshed — ${res.count} apps available`);
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Failed to refresh catalog');
		} finally {
			refreshing = false;
		}
	}

	function handleInstalled() {
		installTarget = null;
		void loadContainers();
	}

	async function handleRemove(app: CatalogApp) {
		if (!selectedNodeId) return;
		if (!confirm(`Remove ${app.name}? Its container will be deleted (data volumes are kept on disk).`)) return;
		busyAppId = app.id;
		error = null;
		try {
			await removeApp(selectedNodeId, app.id);
			await loadContainers();
			toastSuccess(`${app.name} removed`);
		} catch (err) {
			const message = err instanceof Error ? err.message : `Failed to remove ${app.name}`;
			error = message;
			toastError(message);
		} finally {
			busyAppId = null;
		}
	}
</script>

<TopBar title="App Store">
	{#if connectedNodes.length > 0}
		<select
			bind:value={selectedNodeId}
			class="rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
		>
			{#each connectedNodes as node (node.id)}
				<option value={node.id}>{node.name}</option>
			{/each}
		</select>
	{/if}
	<button
		onclick={handleRefreshCatalog}
		disabled={refreshing}
		title="Re-fetch the Unraid Community Applications catalog"
		class="rounded-xl border border-[var(--border)] px-3 py-2 text-sm font-semibold text-[var(--fg-muted)] hover:bg-[var(--track)] hover:text-[var(--fg)] disabled:opacity-50"
	>
		{refreshing ? 'Refreshing…' : '↻ Refresh catalog'}
	</button>
</TopBar>

<main class="flex-1 p-6">
	{#if error}
		<div class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-500">{error}</div>
	{/if}

	{#if connectedNodes.length === 0}
		<div class="mb-4 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-4 py-3 text-sm text-[var(--fg-muted)]">
			No connected servers yet — you can browse the catalog, but deploying needs a connected server.
		</div>
	{/if}

	<div class="mb-4 flex flex-col gap-3">
		<div class="relative">
			<svg viewBox="0 0 24 24" class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--fg-subtle)]" fill="none" stroke="currentColor" stroke-width="2">
				<circle cx="11" cy="11" r="7" />
				<path d="m20 20-3-3" stroke-linecap="round" />
			</svg>
			<input
				bind:value={search}
				placeholder="Search {apps.length.toLocaleString()} apps…"
				class="w-full rounded-xl border border-[var(--border)] bg-[var(--surface)] py-2.5 pl-9 pr-3 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
			/>
		</div>

		<div class="flex gap-2 overflow-x-auto pb-1">
			<button
				onclick={() => (activeCategory = 'All')}
				class="shrink-0 rounded-full px-3.5 py-1.5 text-xs font-semibold transition-colors
					{activeCategory === 'All' ? 'bg-[var(--accent)] text-[var(--accent-fg)]' : 'bg-[var(--track)] text-[var(--fg-muted)] hover:text-[var(--fg)]'}"
			>
				All apps
			</button>
			{#each categories as cat (cat)}
				<button
					onclick={() => (activeCategory = cat)}
					class="shrink-0 rounded-full px-3.5 py-1.5 text-xs font-semibold transition-colors
						{activeCategory === cat ? 'bg-[var(--accent)] text-[var(--accent-fg)]' : 'bg-[var(--track)] text-[var(--fg-muted)] hover:text-[var(--fg)]'}"
				>
					{cat}
				</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">Loading catalog…</p>
		</div>
	{:else if filteredApps.length === 0}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No apps match "{search}".</p>
		</div>
	{:else}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
			{#each visibleApps as app (app.id)}
				{@const status = selectedNodeId ? statusFor(app.id) : 'not-installed'}
				<article class="flex gap-3 rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-4 transition-shadow hover:shadow-md">
					<AppIcon name={app.name} icon={app.icon} size={56} />

					<div class="flex min-w-0 flex-1 flex-col">
						<div class="flex items-start justify-between gap-2">
							<h2 class="truncate font-semibold text-[var(--fg)]">{app.name}</h2>
							{#if status === 'running'}
								<span class="mt-0.5 h-2 w-2 shrink-0 rounded-full bg-[var(--accent)]" title="Running"></span>
							{:else if status === 'stopped'}
								<span class="mt-0.5 h-2 w-2 shrink-0 rounded-full bg-[var(--fg-subtle)]" title="Stopped"></span>
							{/if}
						</div>
						<p class="mb-2 text-xs text-[var(--fg-subtle)]">{app.category ?? 'Other'}</p>
						<p class="line-clamp-2 flex-1 text-xs leading-5 text-[var(--fg-muted)]">{app.description}</p>

						{#if status === 'not-installed'}
							<button
								type="button"
								onclick={() => (installTarget = app)}
								disabled={!selectedNodeId}
								class="mt-3 self-start rounded-lg bg-[var(--accent)] px-3 py-1.5 text-xs font-semibold text-[var(--accent-fg)] disabled:cursor-not-allowed disabled:opacity-50"
							>
								Install
							</button>
						{:else}
							<button
								type="button"
								onclick={() => handleRemove(app)}
								disabled={busyAppId === app.id}
								class="mt-3 self-start rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs font-semibold text-rose-500 hover:bg-rose-500/10 disabled:cursor-not-allowed disabled:opacity-50"
							>
								{busyAppId === app.id ? 'Removing…' : 'Remove'}
							</button>
						{/if}
					</div>
				</article>
			{/each}
		</div>

		{#if visibleCount < filteredApps.length}
			<div class="mt-6 flex justify-center">
				<button
					onclick={() => (visibleCount += PAGE_SIZE)}
					class="rounded-xl border border-[var(--border)] px-5 py-2.5 text-sm font-semibold text-[var(--fg-muted)] hover:bg-[var(--track)] hover:text-[var(--fg)]"
				>
					Show more ({filteredApps.length - visibleCount} left)
				</button>
			</div>
		{/if}
	{/if}
</main>

{#if installTarget && selectedNodeId}
	<InstallAppModal app={installTarget} nodeId={selectedNodeId} onClose={() => (installTarget = null)} onDeployed={handleInstalled} />
{/if}
