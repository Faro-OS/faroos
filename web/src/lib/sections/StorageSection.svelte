<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import StatGauge from '$lib/components/StatGauge.svelte';
	import { formatBytes } from '$lib/format';
	import { listNodes, type Node } from '$lib/api';

	let nodes = $state<Node[]>([]);
	let loadError = $state<string | null>(null);

	const connectedNodes = $derived(nodes.filter((n) => n.connected));

	async function refresh() {
		try {
			nodes = await listNodes();
			loadError = null;
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to reach the FaroOS server';
		}
	}

	$effect(() => {
		refresh();
		const interval = setInterval(refresh, 5000);
		return () => clearInterval(interval);
	});
</script>

<TopBar title="Storage" />

<main class="flex-1 p-6">
	{#if loadError}
		<div class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-500">{loadError}</div>
	{/if}

	{#if connectedNodes.length === 0}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No connected servers yet.</p>
		</div>
	{:else}
		<div class="flex flex-col gap-4">
			{#each connectedNodes as node (node.id)}
				<div class="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
					<h2 class="mb-4 font-semibold text-[var(--fg)]">{node.name}</h2>
					{#if !node.stats.disks || node.stats.disks.length === 0}
						<p class="text-sm text-[var(--fg-subtle)]">No disk information reported yet.</p>
					{:else}
						<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
							{#each node.stats.disks as disk (disk.mountPoint)}
								<div class="rounded-xl border border-[var(--border)] p-4">
									<div class="mb-2 flex items-center justify-between">
										<span class="font-mono text-sm font-medium text-[var(--fg)]">{disk.mountPoint}</span>
										<span class="text-xs uppercase text-[var(--fg-subtle)]">{disk.filesystem}</span>
									</div>
									<StatGauge
										label="Used"
										percent={disk.totalBytes ? (disk.usedBytes / disk.totalBytes) * 100 : 0}
										detail="{formatBytes(disk.usedBytes)} / {formatBytes(disk.totalBytes)}"
									/>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</main>
