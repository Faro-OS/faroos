<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import {
		deployApp,
		listApps,
		listContainers,
		listNodes,
		removeApp,
		type CatalogApp,
		type Container,
		type Node
	} from '$lib/api';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let apps = $state<CatalogApp[]>([]);
	let containers = $state<Container[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let busyAppId = $state<string | null>(null);

	const connectedNodes = $derived(nodes.filter((n) => n.connected));

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
			apps = await listApps();
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

	async function handleDeploy(app: CatalogApp) {
		if (!selectedNodeId) return;
		busyAppId = app.id;
		error = null;
		try {
			await deployApp(selectedNodeId, app.id);
			await loadContainers();
			toastSuccess(`${app.name} deployed`);
		} catch (err) {
			const message = err instanceof Error ? err.message : `Failed to deploy ${app.name}`;
			error = message;
			toastError(message);
		} finally {
			busyAppId = null;
		}
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

	{#if loading}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">Loading catalog…</p>
		</div>
	{:else}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
			{#each apps as app (app.id)}
				{@const status = selectedNodeId ? statusFor(app.id) : 'not-installed'}
				<article class="flex min-h-48 flex-col rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
					<div class="mb-4 flex items-center justify-between">
						<div class="grid h-11 w-11 place-items-center rounded-xl bg-[var(--track)] text-lg font-semibold text-[var(--accent)]">
							{app.name.charAt(0)}
						</div>
						{#if status === 'running'}
							<span class="inline-flex items-center gap-1.5 rounded-full bg-[var(--accent)]/10 px-2.5 py-1 text-xs font-semibold text-[var(--accent)]">
								<span class="h-1.5 w-1.5 rounded-full bg-[var(--accent)]"></span> Running
							</span>
						{:else if status === 'stopped'}
							<span class="inline-flex items-center gap-1.5 rounded-full bg-[var(--track)] px-2.5 py-1 text-xs font-semibold text-[var(--fg-subtle)]">
								<span class="h-1.5 w-1.5 rounded-full bg-[var(--fg-subtle)]"></span> Stopped
							</span>
						{/if}
					</div>
					<h2 class="font-semibold text-[var(--fg)]">{app.name}</h2>
					<p class="mt-1 flex-1 text-sm leading-6 text-[var(--fg-muted)]">{app.description}</p>
					<p class="mt-2 font-mono text-xs text-[var(--fg-subtle)]">{app.image}</p>

					{#if status === 'not-installed'}
						<button
							type="button"
							onclick={() => handleDeploy(app)}
							disabled={!selectedNodeId || busyAppId === app.id}
							class="mt-5 w-full rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)] disabled:cursor-not-allowed disabled:opacity-50"
						>
							{busyAppId === app.id ? 'Deploying… (pulling image)' : 'Deploy'}
						</button>
					{:else}
						<button
							type="button"
							onclick={() => handleRemove(app)}
							disabled={busyAppId === app.id}
							class="mt-5 w-full rounded-xl border border-[var(--border)] px-4 py-2 text-sm font-semibold text-rose-500 hover:bg-rose-500/10 disabled:cursor-not-allowed disabled:opacity-50"
						>
							{busyAppId === app.id ? 'Removing…' : 'Remove'}
						</button>
					{/if}
				</article>
			{/each}
		</div>
	{/if}
</main>
