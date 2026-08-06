<script lang="ts">
	import type { Node } from '$lib/api';
	import { formatBytes, formatRelativeTime, formatUptime } from '$lib/format';
	import StatGauge from './StatGauge.svelte';

	let { node }: { node: Node } = $props();

	const memPercent = $derived(
		node.stats.memTotalBytes ? (node.stats.memUsedBytes / node.stats.memTotalBytes) * 100 : 0
	);
	const diskPercent = $derived(
		node.stats.diskTotalBytes ? (node.stats.diskUsedBytes / node.stats.diskTotalBytes) * 100 : 0
	);
</script>

<div class="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5 shadow-sm transition-shadow hover:shadow-md">
	<div class="mb-4 flex items-center justify-between">
		<div class="flex items-center gap-2.5">
			<span
				class="relative flex h-2.5 w-2.5 shrink-0 rounded-full {node.connected ? 'bg-emerald-500' : 'bg-zinc-400'}"
			>
				{#if node.connected}
					<span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-500 opacity-75"></span>
				{/if}
			</span>
			<h3 class="font-semibold text-[var(--fg)]">{node.name}</h3>
		</div>
		<span class="text-xs text-[var(--fg-subtle)]">
			{node.connected ? `up ${formatUptime(node.stats.uptimeSeconds)}` : `seen ${formatRelativeTime(node.lastSeen)}`}
		</span>
	</div>

	<div class="flex flex-col gap-3.5">
		<StatGauge label="CPU" percent={node.stats.cpuPercent} />
		<StatGauge
			label="Memory"
			percent={memPercent}
			detail="{formatBytes(node.stats.memUsedBytes)} / {formatBytes(node.stats.memTotalBytes)}"
		/>
		<StatGauge
			label="Disk"
			percent={diskPercent}
			detail="{formatBytes(node.stats.diskUsedBytes)} / {formatBytes(node.stats.diskTotalBytes)}"
		/>
	</div>
</div>
