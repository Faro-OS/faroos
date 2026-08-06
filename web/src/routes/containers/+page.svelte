<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
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
	let actingOn = $state<string | null>(null);

	let logsFor = $state<Container | null>(null);
	let logsText = $state('');
	let logsLoading = $state(false);

	const connectedNodes = $derived(nodes.filter((n) => n.connected));

	async function loadNodes() {
		nodes = await listNodes();
		if (!selectedNodeId && connectedNodes.length > 0) {
			selectedNodeId = connectedNodes[0].id;
		}
	}

	async function loadContainers() {
		if (!selectedNodeId) {
			containers = [];
			loading = false;
			return;
		}
		loading = true;
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
	});

	$effect(() => {
		if (selectedNodeId) {
			void loadContainers();
		}
	});

	function containerName(c: Container): string {
		return c.names[0]?.replace(/^\//, '') ?? c.id.slice(0, 12);
	}

	async function runAction(c: Container, action: ContainerAction) {
		if (!selectedNodeId) return;
		actingOn = c.id;
		try {
			await containerAction(selectedNodeId, c.id, action);
			await loadContainers();
			toastSuccess(`${containerName(c)} ${action === 'stop' ? 'stopped' : action === 'start' ? 'started' : 'restarted'}`);
		} catch (err) {
			const message = err instanceof Error ? err.message : `Failed to ${action} container`;
			error = message;
			toastError(message);
		} finally {
			actingOn = null;
		}
	}

	async function openLogs(c: Container) {
		if (!selectedNodeId) return;
		logsFor = c;
		logsLoading = true;
		logsText = '';
		try {
			const res = await containerLogs(selectedNodeId, c.id, 300);
			logsText = res.logs || '(no output)';
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
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No connected servers yet.</p>
		</div>
	{:else if loading}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">Loading containers…</p>
		</div>
	{:else if containers.length === 0}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No containers on this server.</p>
		</div>
	{:else}
		<div class="overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)]">
			<div class="overflow-x-auto">
				<table class="w-full min-w-[900px] text-left text-sm">
					<thead class="border-b border-[var(--border)] bg-[var(--track)] text-xs uppercase tracking-wide text-[var(--fg-subtle)]">
						<tr>
							<th class="px-5 py-3 font-semibold" scope="col">Name</th>
							<th class="px-5 py-3 font-semibold" scope="col">Image</th>
							<th class="px-5 py-3 font-semibold" scope="col">State</th>
							<th class="px-5 py-3 font-semibold" scope="col">Ports</th>
							<th class="px-5 py-3 font-semibold" scope="col">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border)]">
						{#each containers as c (c.id)}
							<tr class="text-[var(--fg-muted)] transition-colors hover:bg-[var(--track)]">
								<th class="whitespace-nowrap px-5 py-4 font-semibold text-[var(--fg)]" scope="row">
									{containerName(c)}
								</th>
								<td class="max-w-[240px] truncate px-5 py-4" title={c.image}>{c.image}</td>
								<td class="whitespace-nowrap px-5 py-4">
									<span class="inline-flex items-center gap-2">
										<span
											class="h-2.5 w-2.5 rounded-full {c.state === 'running' ? 'bg-[var(--accent)]' : 'bg-[var(--fg-subtle)]'}"
										></span>
										{c.status}
									</span>
								</td>
								<td class="whitespace-nowrap px-5 py-4">
									{c.ports
										.filter((p) => p.publicPort)
										.map((p) => `${p.publicPort}→${p.privatePort}`)
										.join(', ') || '—'}
								</td>
								<td class="whitespace-nowrap px-5 py-4">
									<div class="flex gap-2">
										{#if c.state === 'running'}
											<button
												onclick={() => runAction(c, 'stop')}
												disabled={actingOn === c.id}
												class="rounded-lg border border-[var(--border)] px-2.5 py-1 text-xs font-semibold hover:bg-[var(--surface-raised)] disabled:opacity-50"
											>
												Stop
											</button>
											<button
												onclick={() => runAction(c, 'restart')}
												disabled={actingOn === c.id}
												class="rounded-lg border border-[var(--border)] px-2.5 py-1 text-xs font-semibold hover:bg-[var(--surface-raised)] disabled:opacity-50"
											>
												Restart
											</button>
										{:else}
											<button
												onclick={() => runAction(c, 'start')}
												disabled={actingOn === c.id}
												class="rounded-lg border border-[var(--border)] px-2.5 py-1 text-xs font-semibold hover:bg-[var(--surface-raised)] disabled:opacity-50"
											>
												Start
											</button>
										{/if}
										<button
											onclick={() => openLogs(c)}
											class="rounded-lg border border-[var(--border)] px-2.5 py-1 text-xs font-semibold hover:bg-[var(--surface-raised)]"
										>
											Logs
										</button>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</main>

{#if logsFor}
	<div class="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
		<div class="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-xl">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--fg)]">Logs — {containerName(logsFor)}</h2>
				<button onclick={() => (logsFor = null)} class="text-sm text-[var(--fg-subtle)] hover:text-[var(--fg)]">Close</button>
			</div>
			<pre class="flex-1 overflow-auto rounded-xl bg-[var(--bg)] p-4 text-xs whitespace-pre-wrap text-[var(--fg)]">{logsLoading
					? 'Loading…'
					: logsText}</pre>
		</div>
	</div>
{/if}
