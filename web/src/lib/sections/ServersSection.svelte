<script lang="ts">
	import { listNodes, type Node } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import { formatBytes, formatRelativeTime, formatUptime } from '$lib/format';

	let nodes = $state<Node[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);

	function formatPercent(value: number): string {
		return `${value.toFixed(1)}%`;
	}

	function formatUsage(used: number, total: number): string {
		if (!total) return '—';
		return `${formatPercent((used / total) * 100)} · ${formatBytes(used)} / ${formatBytes(total)}`;
	}

	async function refresh() {
		try {
			nodes = await listNodes();
			loadError = null;
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load servers';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void refresh();
		const interval = setInterval(refresh, 4000);
		return () => clearInterval(interval);
	});
</script>

<TopBar title="Servers" />

<main class="flex-1 p-6">
	{#if loadError}
		<div class="mb-4 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-4 py-3 text-sm text-[var(--fg-muted)]">
			{loadError} — is the FaroOS server running?
		</div>
	{/if}

	{#if loading}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">Loading servers…</p>
		</div>
	{:else if nodes.length === 0 && !loadError}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No servers paired yet.</p>
		</div>
	{:else if nodes.length > 0}
		<div class="overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)]">
			<div class="overflow-x-auto">
				<table class="w-full min-w-[900px] text-left text-sm">
					<thead class="border-b border-[var(--border)] bg-[var(--track)] text-xs uppercase tracking-wide text-[var(--fg-subtle)]">
						<tr>
							<th class="px-5 py-3 font-semibold" scope="col">Name</th>
							<th class="px-5 py-3 font-semibold" scope="col">Status</th>
							<th class="px-5 py-3 font-semibold" scope="col">CPU %</th>
							<th class="px-5 py-3 font-semibold" scope="col">Memory</th>
							<th class="px-5 py-3 font-semibold" scope="col">Disk</th>
							<th class="px-5 py-3 font-semibold" scope="col">Uptime</th>
							<th class="px-5 py-3 font-semibold" scope="col">Last seen</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border)]">
						{#each nodes as node (node.id)}
							<tr class="text-[var(--fg-muted)] transition-colors hover:bg-[var(--track)]">
								<th class="whitespace-nowrap px-5 py-4 font-semibold text-[var(--fg)]" scope="row">
									{node.name}
								</th>
								<td class="whitespace-nowrap px-5 py-4">
									<span class="inline-flex items-center gap-2">
										<span
											class="h-2.5 w-2.5 rounded-full {node.connected
												? 'bg-[var(--accent)]'
												: 'bg-[var(--fg-subtle)]'}"
										></span>
										{node.connected ? 'Connected' : 'Disconnected'}
									</span>
								</td>
								<td class="whitespace-nowrap px-5 py-4">{formatPercent(node.stats.cpuPercent)}</td>
								<td class="whitespace-nowrap px-5 py-4">
									{formatUsage(node.stats.memUsedBytes, node.stats.memTotalBytes)}
								</td>
								<td class="whitespace-nowrap px-5 py-4">
									{formatUsage(node.stats.diskUsedBytes, node.stats.diskTotalBytes)}
								</td>
								<td class="whitespace-nowrap px-5 py-4">{formatUptime(node.stats.uptimeSeconds)}</td>
								<td class="whitespace-nowrap px-5 py-4">{formatRelativeTime(node.lastSeen)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</main>
