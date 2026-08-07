<script lang="ts">
	import RadialGauge from '$lib/components/RadialGauge.svelte';
	import AppIcon from '$lib/components/AppIcon.svelte';
	import { formatBytes } from '$lib/format';
	import {
		createPairing,
		listApps,
		listContainers,
		listNodes,
		type CatalogApp,
		type Container,
		type Node,
		type PairingResult
	} from '$lib/api';
	import { toastError } from '$lib/toast.svelte';
	import { setSection } from '$lib/section.svelte';

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let apps = $state<CatalogApp[]>([]);
	let containers = $state<Container[]>([]);
	let loadError = $state<string | null>(null);

	let showPairModal = $state(false);
	let newNodeName = $state('');
	let pairingResult = $state<PairingResult | null>(null);
	let pairing = $state(false);

	let clock = $state(new Date());

	const connectedNodes = $derived(nodes.filter((n) => n.connected));
	const selectedNode = $derived(nodes.find((n) => n.id === selectedNodeId) ?? null);

	const deployedApps = $derived.by(() => {
		return apps
			.map((app) => {
				const target = `/faroos-app-${app.id}`;
				const container = containers.find((c) => c.names.includes(target));
				return { app, container };
			})
			.filter((x) => x.container);
	});

	async function refreshNodes() {
		try {
			nodes = await listNodes();
			loadError = null;
			if (!selectedNodeId && connectedNodes.length > 0) {
				selectedNodeId = connectedNodes[0].id;
			}
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to reach the FaroOS server';
		}
	}

	async function refreshContainers() {
		if (!selectedNodeId) {
			containers = [];
			return;
		}
		try {
			containers = await listContainers(selectedNodeId);
		} catch {
			containers = [];
		}
	}

	$effect(() => {
		(async () => {
			apps = await listApps().catch(() => []);
			await refreshNodes();
			await refreshContainers();
		})();
		const interval = setInterval(() => {
			refreshNodes();
			refreshContainers();
		}, 5000);
		const clockInterval = setInterval(() => (clock = new Date()), 1000);
		return () => {
			clearInterval(interval);
			clearInterval(clockInterval);
		};
	});

	$effect(() => {
		if (selectedNodeId) void refreshContainers();
	});

	function appUrl(hostPort: number): string {
		return `${window.location.protocol}//${window.location.hostname}:${hostPort}`;
	}

	async function submitPairing(e: SubmitEvent) {
		e.preventDefault();
		if (!newNodeName.trim()) return;
		pairing = true;
		try {
			pairingResult = await createPairing(newNodeName.trim());
			refreshNodes();
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to create pairing';
			loadError = message;
			toastError(message);
		} finally {
			pairing = false;
		}
	}

	function closeModal() {
		showPairModal = false;
		newNodeName = '';
		pairingResult = null;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && showPairModal) closeModal();
	}

	const memPercent = $derived(
		selectedNode?.stats.memTotalBytes ? (selectedNode.stats.memUsedBytes / selectedNode.stats.memTotalBytes) * 100 : 0
	);
	const rootDisk = $derived(selectedNode?.stats.disks?.find((d) => d.mountPoint === '/') ?? null);
	const diskPercent = $derived(
		selectedNode?.stats.diskTotalBytes
			? (selectedNode.stats.diskUsedBytes / selectedNode.stats.diskTotalBytes) * 100
			: 0
	);
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="min-h-full" style="background: var(--wallpaper);">
	<div class="mx-auto max-w-6xl px-4 pb-10 pt-6 sm:px-8">
		<div class="mb-6 flex flex-wrap items-center justify-between gap-3">
			<div>
				<p class="text-3xl font-semibold text-white drop-shadow-sm">
					{clock.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
				</p>
				<p class="text-sm text-white/70">{clock.toLocaleDateString(undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}</p>
			</div>

			<div class="flex items-center gap-2">
				{#if nodes.length > 0}
					<select
						bind:value={selectedNodeId}
						class="rounded-xl border border-white/15 bg-black/30 px-3 py-2 text-sm text-white backdrop-blur-md outline-none"
					>
						{#each nodes as node (node.id)}
							<option value={node.id} class="text-black">{node.name}{node.connected ? '' : ' (offline)'}</option>
						{/each}
					</select>
				{/if}
				<button
					onclick={() => (showPairModal = true)}
					class="rounded-xl bg-white/90 px-4 py-2 text-sm font-semibold text-black transition-opacity hover:opacity-90"
				>
					+ Add server
				</button>
			</div>
		</div>

		{#if loadError}
			<div class="mb-4 rounded-xl border border-rose-400/30 bg-rose-950/40 px-4 py-3 text-sm text-rose-200 backdrop-blur-md">
				{loadError} — is the FaroOS server running?
			</div>
		{/if}

		{#if nodes.length === 0}
			<div class="grid place-items-center rounded-2xl border border-dashed border-white/20 bg-black/20 py-24 text-center backdrop-blur-md">
				<p class="text-white/80">No servers paired yet.</p>
				<button onclick={() => (showPairModal = true)} class="mt-3 text-sm font-semibold text-white hover:underline">
					Pair your first server →
				</button>
			</div>
		{:else}
			<div class="grid grid-cols-1 gap-5 lg:grid-cols-[280px_1fr]">
				<!-- Widget column -->
				<div class="flex flex-col gap-4">
					<div class="rounded-2xl border border-white/10 bg-black/30 p-5 backdrop-blur-md">
						<div class="mb-3 flex items-center justify-between">
							<h2 class="text-sm font-semibold text-white">System Status</h2>
							<span class="flex items-center gap-1.5 text-xs text-white/60">
								<span class="h-1.5 w-1.5 rounded-full {selectedNode?.connected ? 'bg-emerald-400' : 'bg-white/40'}"></span>
								{selectedNode?.connected ? 'Online' : 'Offline'}
							</span>
						</div>
						{#if selectedNode}
							<div class="flex justify-around">
								<RadialGauge percent={selectedNode.stats.cpuPercent} label="CPU" sublabel="usage" />
								<RadialGauge percent={memPercent} label="RAM" sublabel={formatBytes(selectedNode.stats.memTotalBytes)} />
							</div>
						{/if}
					</div>

					<div class="rounded-2xl border border-white/10 bg-black/30 p-5 backdrop-blur-md">
						<div class="mb-3 flex items-center justify-between">
							<h2 class="text-sm font-semibold text-white">Storage</h2>
							<button onclick={() => setSection('storage')} class="text-xs text-white/60 hover:text-white">Details →</button>
						</div>
						{#if rootDisk}
							<p class="mb-2 text-xs text-white/60">{formatBytes(rootDisk.usedBytes)} of {formatBytes(rootDisk.totalBytes)} used</p>
							<div class="h-2.5 w-full overflow-hidden rounded-full bg-white/10">
								<div
									class="h-full rounded-full bg-white/90 transition-all duration-500"
									style="width: {diskPercent}%"
								></div>
							</div>
						{:else}
							<p class="text-xs text-white/50">No disk data yet.</p>
						{/if}
					</div>
				</div>

				<!-- Apps grid -->
				<div>
					<h2 class="mb-3 text-sm font-semibold text-white">Apps</h2>
					<div class="grid grid-cols-3 gap-4 sm:grid-cols-4 md:grid-cols-5">
						{#each deployedApps as { app, container } (app.id)}
							{#if container}
								<a
									href={appUrl(app.ports[0]?.host ?? 80)}
									target="_blank"
									rel="noreferrer"
									class="flex flex-col items-center gap-2 rounded-2xl border border-white/10 bg-black/25 p-4 text-center backdrop-blur-md transition-colors hover:bg-black/40"
								>
									<AppIcon name={app.name} icon={app.icon} size={48} />
									<span class="line-clamp-2 text-xs font-medium text-white">{app.name}</span>
									<span class="flex items-center gap-1 text-[10px] text-white/60">
										<span class="h-1.5 w-1.5 rounded-full {container.state === 'running' ? 'bg-emerald-400' : 'bg-white/40'}"></span>
										{container.state === 'running' ? 'Running' : 'Stopped'}
									</span>
								</a>
							{/if}
						{/each}

						<button
							onclick={() => setSection('apps')}
							class="flex flex-col items-center justify-center gap-2 rounded-2xl border border-dashed border-white/25 bg-black/10 p-4 text-center text-white/70 backdrop-blur-md transition-colors hover:bg-black/25 hover:text-white"
						>
							<span class="grid h-12 w-12 place-items-center rounded-2xl border border-white/30 text-2xl">+</span>
							<span class="text-xs font-medium">App Store</span>
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

{#if showPairModal}
	<div class="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4">
		<div class="w-full max-w-md rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-xl">
			{#if !pairingResult}
				<h2 class="mb-1 text-lg font-semibold text-[var(--fg)]">Pair a new server</h2>
				<p class="mb-4 text-sm text-[var(--fg-subtle)]">
					Give it a name. You'll get credentials to run the FaroOS agent on that machine.
				</p>
				<form onsubmit={submitPairing} class="flex flex-col gap-3">
					<input
						bind:value={newNodeName}
						placeholder="e.g. home-server"
						class="rounded-xl border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
					/>
					<div class="flex justify-end gap-2">
						<button type="button" onclick={closeModal} class="rounded-xl px-4 py-2 text-sm text-[var(--fg-muted)] hover:text-[var(--fg)]">
							Cancel
						</button>
						<button
							type="submit"
							disabled={pairing}
							class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)] disabled:opacity-50"
						>
							{pairing ? 'Pairing…' : 'Generate credentials'}
						</button>
					</div>
				</form>
			{:else}
				<h2 class="mb-1 text-lg font-semibold text-[var(--fg)]">Run this on {pairingResult.name}</h2>
				<p class="mb-3 text-sm text-[var(--fg-subtle)]">
					This token is shown once. Copy it into the agent's environment before closing this dialog.
				</p>
				<pre class="overflow-x-auto rounded-xl bg-[var(--bg)] p-4 text-xs text-[var(--fg)]"><code
					>FAROOS_SERVER=ws://&lt;this-server&gt;:8090/api/agent/connect
FAROOS_NODE_ID={pairingResult.id}
FAROOS_TOKEN={pairingResult.token}
./faroos-agent</code
				></pre>
				<div class="mt-4 flex justify-end">
					<button onclick={closeModal} class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)]">
						Done
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
