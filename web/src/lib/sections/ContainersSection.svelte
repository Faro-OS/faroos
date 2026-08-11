<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import AppIcon from '$lib/components/AppIcon.svelte';
	import { containerAppIcon, containerAppName } from '$lib/containerApps';
	import {
		containerAction,
		containerLogs,
		listContainers,
		listNodes,
		type Container,
		type ContainerAction,
		type Node
	} from '$lib/api';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let containers = $state<Container[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let actingOn = $state<{ id: string; action: ContainerAction } | null>(null);
	let logsFor = $state<Container | null>(null);
	let logsText = $state('');
	let logsLoading = $state(false);
	let search = $state('');

	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const running = $derived(containers.filter((container) => container.state === 'running'));
	const publishedPorts = $derived(containers.flatMap((container) => container.ports).filter((port) => port.publicPort).length);
	const filtered = $derived.by(() => {
		const query = search.trim().toLowerCase();
		if (!query) return containers;
		return containers.filter((container) => `${containerName(container)} ${container.image} ${container.status}`.toLowerCase().includes(query));
	});

	async function loadNodes() {
		nodes = await listNodes();
		if (!selectedNodeId && connectedNodes.length > 0) selectedNodeId = connectedNodes[0].id;
	}

	async function loadContainers(showLoader = true) {
		if (!selectedNodeId) {
			containers = [];
			loading = false;
			return;
		}
		if (showLoader) loading = true;
		try {
			containers = await listContainers(selectedNodeId);
			error = null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load containers';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		(async () => {
			await loadNodes();
			await loadContainers();
		})();
		const interval = setInterval(() => void loadContainers(false), 5000);
		return () => clearInterval(interval);
	});

	$effect(() => {
		if (selectedNodeId) void loadContainers();
	});

	function containerName(container: Container): string {
		return container.names[0]?.replace(/^\//, '') ?? container.id.slice(0, 12);
	}

	function shortImage(image: string): string {
		return image.replace(/^docker\.io\//, '').replace(/^library\//, '');
	}

	async function runAction(container: Container, action: ContainerAction) {
		if (!selectedNodeId) return;
		actingOn = { id: container.id, action };
		try {
			await containerAction(selectedNodeId, container.id, action);
			await loadContainers(false);
			toastSuccess(`${containerName(container)} ${action === 'stop' ? 'stopped' : action === 'start' ? 'started' : 'restarted'}`);
		} catch (err) {
			const message = err instanceof Error ? err.message : `Failed to ${action} container`;
			error = message;
			toastError(message);
		} finally {
			actingOn = null;
		}
	}

	async function openLogs(container: Container) {
		if (!selectedNodeId) return;
		logsFor = container;
		logsLoading = true;
		logsText = '';
		try {
			const result = await containerLogs(selectedNodeId, container.id, 300);
			logsText = result.logs || '(no output)';
		} catch (err) {
			logsText = err instanceof Error ? err.message : 'Failed to load logs';
		} finally {
			logsLoading = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && logsFor) logsFor = null;
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<TopBar title="Containers">
	{#if connectedNodes.length > 0}<select bind:value={selectedNodeId} class="control rounded-xl px-3 text-sm text-[var(--fg)] outline-none">{#each connectedNodes as node (node.id)}<option value={node.id}>{node.name}</option>{/each}</select>{/if}
</TopBar>

<main class="section-enter mx-auto w-full max-w-[1480px] p-4 pb-32 sm:p-7 sm:pb-32 lg:p-10 lg:pb-32">
	{#if error}<div class="mb-5 rounded-2xl border border-rose-500/15 bg-rose-500/8 px-4 py-3 text-sm text-rose-500">{error}</div>{/if}

	<div class="mb-6 grid grid-cols-3 gap-3">
		{#each [
			{ label: 'Running', value: running.length, detail: 'Healthy services', color: 'text-emerald-500' },
			{ label: 'Stopped', value: containers.length - running.length, detail: 'Waiting to start', color: 'text-[var(--fg-muted)]' },
			{ label: 'Published ports', value: publishedPorts, detail: 'Network endpoints', color: 'text-blue-500' }
		] as stat (stat.label)}
			<div class="surface-card rounded-[20px] p-4 sm:p-5"><p class="eyebrow mb-3">{stat.label}</p><p class="text-2xl font-semibold tracking-tight {stat.color}">{stat.value}</p><p class="mt-1 hidden text-[11px] text-[var(--fg-subtle)] sm:block">{stat.detail}</p></div>
		{/each}
	</div>

	<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">Workloads</h2><p class="mt-1 text-xs text-[var(--fg-subtle)]">Live Docker services on the selected server</p></div>
		<label class="control flex h-10 items-center gap-2 rounded-xl px-3 sm:w-64" aria-label="Filter containers"><svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0 text-[var(--fg-subtle)]" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3-3" /></svg><input bind:value={search} placeholder="Filter services…" class="min-w-0 flex-1 bg-transparent text-sm text-[var(--fg)] outline-none" /></label>
	</div>

	{#if connectedNodes.length === 0}
		<div class="surface-card grid min-h-72 place-items-center rounded-[24px] border-dashed text-sm text-[var(--fg-muted)]">No connected servers yet.</div>
	{:else if loading}
		<div class="space-y-3">{#each [1, 2, 3] as item (item)}<div class="surface-card h-24 animate-pulse rounded-[22px]"></div>{/each}</div>
	{:else if containers.length === 0}
		<div class="surface-card grid min-h-72 place-items-center rounded-[24px] border-dashed text-center"><div><p class="font-medium text-[var(--fg)]">A beautifully clean slate.</p><p class="mt-1 text-sm text-[var(--fg-subtle)]">Install an app to create your first container.</p></div></div>
	{:else}
		<div class="surface-card overflow-hidden rounded-[24px]">
			{#each filtered as container, index (container.id)}
				<article class="group flex flex-col gap-4 p-4 transition-colors hover:bg-[var(--track)] sm:flex-row sm:items-center sm:p-5 {index > 0 ? 'border-t border-[var(--border)]' : ''}">
					<AppIcon name={containerAppName(container)} icon={containerAppIcon(container)} size={48} />
					<div class="min-w-0 flex-1"><div class="flex items-center gap-2"><h3 class="truncate text-sm font-semibold text-[var(--fg)]">{containerName(container)}</h3><span class="h-1.5 w-1.5 rounded-full {container.state === 'running' ? 'status-dot bg-emerald-500 text-emerald-500' : 'bg-[var(--fg-subtle)]'}"></span></div><p class="mt-1 truncate text-xs text-[var(--fg-subtle)]" title={container.image}>{shortImage(container.image)}</p></div>
					<div class="flex min-w-[150px] flex-col"><span class="text-xs font-medium text-[var(--fg-muted)]">{container.status}</span><span class="mt-1 text-[10px] text-[var(--fg-subtle)]">{container.ports.filter((port) => port.publicPort).map((port) => `${port.publicPort} → ${port.privatePort}`).join('  ·  ') || 'No public ports'}</span></div>
					<div class="flex flex-wrap items-center gap-1.5">
						<button type="button" onclick={() => openLogs(container)} class="control h-9 min-h-0 rounded-xl px-3 text-xs font-medium text-[var(--fg-muted)]">Logs</button>
						{#if container.state === 'running'}
							<button type="button" onclick={() => runAction(container, 'restart')} disabled={actingOn?.id === container.id} class="control flex h-9 min-h-0 items-center gap-1.5 rounded-xl px-3 text-xs font-semibold text-[var(--fg-muted)] disabled:opacity-55" aria-label="Restart"><svg viewBox="0 0 24 24" class="h-3.5 w-3.5 {actingOn?.id === container.id && actingOn.action === 'restart' ? 'animate-spin' : ''}" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M20 7v5h-5M4 17v-5h5M6.1 9a7 7 0 0 1 11.6-2L20 9M4 15l2.3 2a7 7 0 0 0 11.6-2" stroke-linecap="round" stroke-linejoin="round" /></svg>{actingOn?.id === container.id && actingOn.action === 'restart' ? 'Restarting…' : 'Restart'}</button>
							<button type="button" onclick={() => runAction(container, 'stop')} disabled={actingOn?.id === container.id} class="control flex h-9 min-h-0 items-center gap-1.5 rounded-xl px-3 text-xs font-semibold text-[var(--fg-muted)] disabled:opacity-55" aria-label="Stop"><svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="currentColor"><rect x="7" y="7" width="10" height="10" rx="1.5" /></svg>{actingOn?.id === container.id && actingOn.action === 'stop' ? 'Stopping…' : 'Stop'}</button>
						{:else}
							<button type="button" onclick={() => runAction(container, 'start')} disabled={actingOn?.id === container.id} class="primary-control flex h-9 items-center gap-1.5 rounded-xl px-3 text-xs font-semibold disabled:opacity-55" aria-label="Start"><svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="currentColor"><path d="m8 5 11 7-11 7V5Z" /></svg>{actingOn?.id === container.id ? 'Starting…' : 'Start'}</button>
						{/if}
					</div>
				</article>
			{/each}
		</div>
	{/if}
</main>

{#if logsFor}
	<div class="logs-layer fixed inset-0 z-[70] flex items-center justify-center p-4"><button type="button" class="logs-scrim absolute inset-0 cursor-default" aria-label="Close logs" onclick={() => (logsFor = null)}></button>
		<div class="logs-panel glass relative flex max-h-[82vh] w-full max-w-4xl flex-col overflow-hidden rounded-[28px]">
			<header class="flex items-center justify-between border-b border-[var(--border)] px-5 py-4"><div><p class="eyebrow mb-1.5">Container output</p><h2 class="text-sm font-semibold text-[var(--fg)]">{containerName(logsFor)}</h2></div><button onclick={() => (logsFor = null)} class="control grid h-9 w-9 min-h-0 place-items-center rounded-xl text-[var(--fg-muted)]" aria-label="Close"><svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 6 12 12M18 6 6 18" /></svg></button></header>
			<pre class="m-3 flex-1 overflow-auto rounded-2xl border border-white/5 bg-[#080a0f] p-5 text-xs leading-6 whitespace-pre-wrap text-zinc-300">{logsLoading ? 'Loading output…' : logsText}</pre>
		</div>
	</div>
{/if}

<style>
	.logs-layer { perspective:1000px; }
	.logs-scrim { border:0; background:color-mix(in srgb,#07080b 42%,transparent); animation:logs-scrim-in 180ms ease-out both; backdrop-filter:blur(9px) saturate(.85); -webkit-backdrop-filter:blur(9px) saturate(.85); }
	.logs-panel { transform-origin:center 25%; animation:logs-panel-in 340ms var(--motion-settle) both; }
	@keyframes logs-scrim-in { from { opacity:0; } to { opacity:1; } }
	@keyframes logs-panel-in { from { opacity:0; transform:translateY(11px) scale(.97); filter:blur(7px); } to { opacity:1; transform:translateY(0) scale(1); filter:blur(0); } }
	@media (prefers-reduced-motion:reduce) { .logs-scrim,.logs-panel { animation:none; } }
	@media (prefers-reduced-transparency:reduce) { .logs-scrim { backdrop-filter:none; -webkit-backdrop-filter:none; } }
</style>
