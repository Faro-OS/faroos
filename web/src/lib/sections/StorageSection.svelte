<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import { formatStorage, totalStorageBytes } from '$lib/format';
	import { listNodes, type Disk, type Node } from '$lib/api';

	let nodes = $state<Node[]>([]);
	let loadError = $state<string | null>(null);
	let selectedNodeId = $state<string | null>(null);
	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const selectedNode = $derived(connectedNodes.find((node) => node.id === selectedNodeId) ?? connectedNodes[0] ?? null);
	const disks = $derived(selectedNode?.stats.disks ?? []);
	const totalCapacity = $derived(selectedNode ? totalStorageBytes(selectedNode.stats) : 0);
	const usedCapacity = $derived(disks.length ? disks.reduce((sum, disk) => sum + disk.usedBytes, 0) : (selectedNode?.stats.diskUsedBytes ?? 0));
	const measuredCapacity = $derived(disks.length ? disks.filter((disk) => disk.filesystem !== 'unmounted').reduce((sum, disk) => sum + disk.totalBytes, 0) : totalCapacity);
	const freeCapacity = $derived(Math.max(0, measuredCapacity - usedCapacity));

	function percent(disk: Disk): number {
		return disk.totalBytes ? (disk.usedBytes / disk.totalBytes) * 100 : 0;
	}

	async function refresh() {
		try {
			nodes = await listNodes();
			if (!selectedNodeId && connectedNodes.length > 0) selectedNodeId = connectedNodes[0].id;
			loadError = null;
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to reach the FaroOS server';
		}
	}

	$effect(() => {
		void refresh();
		const interval = setInterval(refresh, 5000);
		return () => clearInterval(interval);
	});
</script>

<TopBar title="Storage">
	{#if connectedNodes.length > 0}<select bind:value={selectedNodeId} class="control rounded-xl px-3 text-sm text-[var(--fg)] outline-none">{#each connectedNodes as node (node.id)}<option value={node.id}>{node.name}</option>{/each}</select>{/if}
</TopBar>

<main class="section-enter mx-auto w-full max-w-[1480px] p-4 pb-32 sm:p-7 sm:pb-32 lg:p-10 lg:pb-32">
	{#if loadError}<div class="mb-5 rounded-2xl border border-rose-500/15 bg-rose-500/8 px-4 py-3 text-sm text-rose-500">{loadError}</div>{/if}

	{#if connectedNodes.length === 0}
		<div class="surface-card grid min-h-80 place-items-center rounded-[24px] border-dashed text-sm text-[var(--fg-muted)]">No connected servers yet.</div>
	{:else}
		<section class="premium-card relative mb-6 overflow-hidden rounded-[28px] p-5 sm:p-7">
			<div class="pointer-events-none absolute -right-20 -top-28 h-72 w-72 rounded-full bg-blue-500/10 blur-[80px]"></div>
			<div class="relative grid gap-8 lg:grid-cols-[1fr_1.4fr] lg:items-end">
				<div><div class="mb-5 flex items-center gap-3"><span class="grid h-12 w-12 place-items-center rounded-[17px] bg-blue-500/10 text-blue-500"><svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M4 7a3 3 0 0 1 3-3h10a3 3 0 0 1 3 3v10a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3V7Z" /><path d="M8 9h.01M8 15h.01" stroke-linecap="round" /></svg></span><div><p class="eyebrow mb-1.5">Total storage</p><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">{selectedNode?.name}</h2></div></div><p class="text-4xl font-semibold tracking-[-0.055em] text-[var(--fg)] sm:text-5xl">{formatStorage(totalCapacity)}</p><p class="mt-2 text-sm text-[var(--fg-subtle)]">across {disks.length || 1} detected {disks.length === 1 ? 'device' : 'devices'}</p></div>
				<div><div class="mb-3 flex items-end justify-between"><div><p class="text-xs font-medium text-[var(--fg-muted)]">Mounted capacity</p><p class="mt-1 text-[11px] text-[var(--fg-subtle)]">{formatStorage(usedCapacity)} used</p></div><div class="text-right"><p class="text-lg font-semibold text-[var(--fg)]">{formatStorage(freeCapacity)}</p><p class="text-[10px] text-[var(--fg-subtle)]">available</p></div></div><div class="h-3 overflow-hidden rounded-full bg-[var(--track)]"><div class="h-full rounded-full bg-gradient-to-r from-blue-500 to-violet-500 transition-all duration-700" style="width:{measuredCapacity ? (usedCapacity / measuredCapacity) * 100 : 0}%"></div></div></div>
			</div>
		</section>

		<div class="mb-4"><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">Storage devices and volumes</h2><p class="mt-1 text-xs text-[var(--fg-subtle)]">Mounted filesystems and detected removable devices</p></div>
		{#if disks.length === 0}
			<div class="surface-card grid min-h-64 place-items-center rounded-[24px] border-dashed text-sm text-[var(--fg-subtle)]">No disk information reported yet.</div>
		{:else}
			<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
				{#each disks as disk (disk.mountPoint)}
					<article class="premium-card rounded-[24px] p-5 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-[var(--shadow-md)]">
						<header class="mb-6 flex items-start justify-between"><div class="flex min-w-0 items-center gap-3"><span class="grid h-10 w-10 shrink-0 place-items-center rounded-[14px] bg-[var(--track)] text-[var(--fg-muted)]"><svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="4" y="5" width="16" height="14" rx="3" /><circle cx="12" cy="12" r="2" /></svg></span><div class="min-w-0"><h3 class="truncate font-mono text-sm font-semibold text-[var(--fg)]">{disk.mountPoint}</h3><p class="mt-1 text-[10px] uppercase tracking-wider text-[var(--fg-subtle)]">{disk.filesystem || 'filesystem'}</p></div></div><span class="rounded-full bg-[var(--track)] px-2.5 py-1 text-[10px] font-semibold {disk.filesystem === 'unmounted' ? 'text-amber-500' : percent(disk) > 90 ? 'text-rose-500' : 'text-emerald-500'}">{disk.filesystem === 'unmounted' ? 'Not mounted' : percent(disk) > 90 ? 'Almost full' : 'Healthy'}</span></header>
						<div class="mb-2 flex items-end justify-between">{#if disk.filesystem === 'unmounted'}<p class="text-2xl font-semibold tracking-tight text-[var(--fg)]">{formatStorage(disk.totalBytes)}<span class="text-xs font-medium text-[var(--fg-subtle)]"> capacity</span></p><p class="text-xs text-[var(--fg-subtle)]">usage unavailable</p>{:else}<p class="text-2xl font-semibold tracking-tight text-[var(--fg)]">{percent(disk).toFixed(0)}<span class="text-xs font-medium text-[var(--fg-subtle)]">% used</span></p><p class="text-xs text-[var(--fg-subtle)]">{formatStorage(disk.totalBytes - disk.usedBytes)} free</p>{/if}</div>
						<div class="h-2 overflow-hidden rounded-full bg-[var(--track)]"><div class="h-full rounded-full transition-all duration-700 {percent(disk) > 90 ? 'bg-rose-500' : percent(disk) > 72 ? 'bg-amber-500' : 'bg-blue-500'}" style="width:{percent(disk)}%"></div></div>
						<div class="mt-4 flex items-center justify-between border-t border-[var(--border)] pt-4 text-[11px] text-[var(--fg-subtle)]"><span>{disk.filesystem === 'unmounted' ? 'Mount to inspect usage' : `${formatStorage(disk.usedBytes)} used`}</span><span>{formatStorage(disk.totalBytes)} total</span></div>
					</article>
				{/each}
			</div>
		{/if}
	{/if}
</main>
