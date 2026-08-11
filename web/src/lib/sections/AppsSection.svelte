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
	import { containerAppIcon, containerAppName, containerWebPort } from '$lib/containerApps';

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
	const installedCount = $derived(containers.filter((container) => container.names.some((name) => name.startsWith('/faroos-app-'))).length);
	const importedContainers = $derived(
		containers.filter(
			(container) =>
				!container.names.some((name) => name.startsWith('/faroos-app-')) &&
				containerWebPort(container)
		)
	);

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

	function importedName(container: Container): string {
		return containerAppName(container);
	}

	function importedIcon(container: Container): string | undefined {
		return containerAppIcon(container);
	}

	function importedUrl(container: Container): string | undefined {
		const port = containerWebPort(container);
		return port ? `${window.location.protocol}//${window.location.hostname}:${port}` : undefined;
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
	{#if connectedNodes.length > 0}<select bind:value={selectedNodeId} class="control rounded-xl px-3 text-sm text-[var(--fg)] outline-none">{#each connectedNodes as node (node.id)}<option value={node.id}>{node.name}</option>{/each}</select>{/if}
	<button onclick={handleRefreshCatalog} disabled={refreshing} title="Refresh the app catalog" class="control rounded-xl px-3 text-xs font-semibold text-[var(--fg-muted)] disabled:opacity-50">{refreshing ? 'Refreshing…' : '↻ Refresh'}</button>
</TopBar>

<main class="section-enter mx-auto w-full max-w-[1480px] p-4 pb-32 sm:p-7 sm:pb-32 lg:p-10 lg:pb-32">
	{#if error}<div class="mb-5 rounded-2xl border border-rose-500/15 bg-rose-500/8 px-4 py-3 text-sm text-rose-500">{error}</div>{/if}
	{#if connectedNodes.length === 0}<div class="mb-5 rounded-2xl border border-amber-500/15 bg-amber-500/8 px-4 py-3 text-sm text-amber-600">Browse freely — connect a server when you are ready to install.</div>{/if}

	<section class="premium-card relative mb-6 overflow-hidden rounded-[28px] p-5 sm:p-7">
		<div class="pointer-events-none absolute -right-20 -top-28 h-72 w-72 rounded-full bg-[var(--fg)]/5 blur-[80px]"></div>
		<div class="relative flex flex-col justify-between gap-6 lg:flex-row lg:items-end">
			<div class="max-w-xl"><p class="eyebrow mb-3">FaroOS App Store</p><h2 class="text-3xl font-semibold tracking-[-0.05em] text-[var(--fg)] sm:text-4xl">Make your server<br /><span class="text-[var(--fg-subtle)]">do almost anything.</span></h2><p class="mt-4 text-sm leading-6 text-[var(--fg-muted)]">Discover self-hosted tools, media servers, smart home platforms and private cloud apps. Deployed in a tap.</p></div>
			<div class="flex gap-3"><div class="rounded-2xl bg-[var(--track)] px-4 py-3"><p class="text-2xl font-semibold tracking-tight text-[var(--fg)]">{apps.length.toLocaleString()}</p><p class="text-[10px] text-[var(--fg-subtle)]">Available apps</p></div><div class="rounded-2xl bg-[var(--track)] px-4 py-3"><p class="text-2xl font-semibold tracking-tight text-[var(--fg)]">{installedCount}</p><p class="text-[10px] text-[var(--fg-subtle)]">Installed here</p></div></div>
		</div>
	</section>

	{#if importedContainers.length > 0}
		<section class="mb-6">
			<div class="mb-3 flex items-end justify-between"><div><p class="eyebrow mb-2">Detected automatically</p><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">Apps already running in Docker</h2></div><span class="text-[11px] text-[var(--fg-subtle)]">{importedContainers.length} imported</span></div>
			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
				{#each importedContainers as container (container.id)}
					{@const url = importedUrl(container)}
					<svelte:element this={url ? 'a' : 'div'} href={url} target={url ? '_blank' : undefined} rel={url ? 'noreferrer' : undefined} class="surface-card group flex items-center gap-3 rounded-[18px] p-3.5 transition-colors hover:border-[var(--border-strong)]">
						<AppIcon name={importedName(container)} icon={importedIcon(container)} size={42} />
						<span class="min-w-0 flex-1"><span class="block truncate text-xs font-semibold text-[var(--fg)]">{importedName(container)}</span><span class="mt-1 block truncate text-[10px] text-[var(--fg-subtle)]">{container.image}</span></span>
						<span class="flex items-center gap-1 text-[10px] font-medium text-[var(--fg-subtle)]"><span class="h-1.5 w-1.5 rounded-full {container.state === 'running' ? 'bg-emerald-500' : 'bg-[var(--fg-subtle)]'}"></span>{container.state === 'running' ? 'Open' : 'Stopped'}</span>
					</svelte:element>
				{/each}
			</div>
		</section>
	{/if}

	<div class="sticky top-[78px] z-20 mb-6 rounded-[22px] border border-[var(--border)] bg-[color-mix(in_srgb,var(--bg)_86%,transparent)] p-2.5 shadow-[var(--shadow-sm)] backdrop-blur-2xl">
		<div class="relative"><svg viewBox="0 0 24 24" class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--fg-subtle)]" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3-3" /></svg><input bind:value={search} placeholder="Search {apps.length.toLocaleString()} apps, categories and possibilities…" class="control h-12 w-full rounded-[15px] pl-10 pr-4 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]" /></div>
		<div class="mt-2 flex gap-1.5 overflow-x-auto pb-0.5"><button onclick={() => (activeCategory = 'All')} class="shrink-0 rounded-xl px-3.5 py-2 text-[11px] font-semibold transition-colors {activeCategory === 'All' ? 'bg-[var(--fg)] text-[var(--bg)]' : 'text-[var(--fg-muted)] hover:bg-[var(--track)] hover:text-[var(--fg)]'}">All apps</button>{#each categories as category (category)}<button onclick={() => (activeCategory = category)} class="shrink-0 rounded-xl px-3.5 py-2 text-[11px] font-semibold transition-colors {activeCategory === category ? 'bg-[var(--fg)] text-[var(--bg)]' : 'text-[var(--fg-muted)] hover:bg-[var(--track)] hover:text-[var(--fg)]'}">{category}</button>{/each}</div>
	</div>

	{#if loading}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{#each [1,2,3,4,5,6,7,8] as item (item)}<div class="surface-card h-44 animate-pulse rounded-[22px]"></div>{/each}</div>
	{:else if filteredApps.length === 0}
		<div class="surface-card grid min-h-72 place-items-center rounded-[24px] border-dashed text-sm text-[var(--fg-muted)]">No apps match “{search}”.</div>
	{:else}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
			{#each visibleApps as app (app.id)}
				{@const status = selectedNodeId ? statusFor(app.id) : 'not-installed'}
				<article class="premium-card group flex min-h-48 gap-4 rounded-[22px] p-4 transition-all duration-300 hover:-translate-y-0.5 hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-md)]">
					<div class="shrink-0 transition-transform duration-300 group-hover:scale-105"><AppIcon name={app.name} icon={app.icon} size={56} /></div>
					<div class="flex min-w-0 flex-1 flex-col"><div class="flex items-start justify-between gap-2"><div class="min-w-0"><h2 class="truncate text-sm font-semibold text-[var(--fg)]">{app.name}</h2><p class="mt-1 text-[10px] font-medium uppercase tracking-wider text-[var(--fg-subtle)]">{app.category ?? 'Other'}</p></div>{#if status !== 'not-installed'}<span class="h-2 w-2 shrink-0 rounded-full {status === 'running' ? 'status-dot bg-emerald-500 text-emerald-500' : 'bg-[var(--fg-subtle)]'}" title={status}></span>{/if}</div>
						<p class="mt-3 line-clamp-3 flex-1 text-xs leading-5 text-[var(--fg-muted)]">{app.description}</p>
						{#if status === 'not-installed'}<button type="button" onclick={() => (installTarget = app)} disabled={!selectedNodeId} class="primary-control mt-3 self-start rounded-xl px-3.5 py-2 text-[11px] font-semibold disabled:cursor-not-allowed disabled:opacity-45">Get</button>{:else}<button type="button" onclick={() => handleRemove(app)} disabled={busyAppId === app.id} class="control mt-3 self-start rounded-xl px-3 py-2 text-[11px] font-semibold text-[var(--fg-muted)] hover:text-rose-500 disabled:opacity-45">{busyAppId === app.id ? 'Removing…' : 'Installed · Remove'}</button>{/if}
					</div>
				</article>
			{/each}
		</div>
		{#if visibleCount < filteredApps.length}<div class="mt-8 flex justify-center"><button onclick={() => (visibleCount += PAGE_SIZE)} class="control rounded-xl px-5 py-2.5 text-sm font-semibold text-[var(--fg-muted)]">Show {Math.min(PAGE_SIZE, filteredApps.length - visibleCount)} more</button></div>{/if}
	{/if}
</main>

{#if installTarget && selectedNodeId}
	<InstallAppModal app={installTarget} nodeId={selectedNodeId} onClose={() => (installTarget = null)} onDeployed={handleInstalled} />
{/if}
