<script lang="ts">
	import type { Node } from '$lib/api';
	import { formatBytes, formatRelativeTime, formatUptime } from '$lib/format';

	let { node, onShowCommand }: { node: Node; onShowCommand?: (node: Node) => void } = $props();
	const hasConnectedBefore = $derived(new Date(node.lastSeen).getTime() > 0);
	const awaitingInstallation = $derived(!node.connected && !hasConnectedBefore);
	const memPercent = $derived(node.stats.memTotalBytes ? (node.stats.memUsedBytes / node.stats.memTotalBytes) * 100 : 0);
	const diskPercent = $derived(node.stats.diskTotalBytes ? (node.stats.diskUsedBytes / node.stats.diskTotalBytes) * 100 : 0);
	const metrics = $derived([
		{ label: 'CPU', value: node.stats.cpuPercent, detail: 'Processor', color: 'bg-blue-500' },
		{ label: 'Memory', value: memPercent, detail: `${formatBytes(node.stats.memUsedBytes)} used`, color: 'bg-violet-500' },
		{ label: 'Storage', value: diskPercent, detail: `${formatBytes(node.stats.diskUsedBytes)} used`, color: 'bg-emerald-500' }
	]);
</script>

<article class="premium-card group rounded-[24px] p-5 sm:p-6">
	<header class="mb-6 flex items-start justify-between gap-4">
		<div class="flex min-w-0 items-center gap-3">
			<span class="grid h-11 w-11 shrink-0 place-items-center rounded-[15px] bg-[var(--track)] text-[var(--fg-muted)]"><svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="4" y="5" width="16" height="6" rx="2" /><rect x="4" y="13" width="16" height="6" rx="2" /><path d="M8 8h.01M8 16h.01" /></svg></span>
			<div class="min-w-0"><h3 class="truncate text-base font-semibold tracking-tight text-[var(--fg)]">{node.name}</h3><p class="mt-1 text-[11px] text-[var(--fg-subtle)]">{node.connected ? `${node.transport === 'direct-p2p' ? 'P2P directo' : 'Relay'} · Up for ${formatUptime(node.stats.uptimeSeconds)}` : awaitingInstallation ? 'Pendiente de instalar' : `Last seen ${formatRelativeTime(node.lastSeen)}`}</p></div>
		</div>
		<span class="flex shrink-0 items-center gap-2 rounded-full px-2.5 py-1 text-[10px] font-semibold {node.connected ? 'bg-emerald-500/10 text-emerald-500' : 'bg-[var(--track)] text-[var(--fg-subtle)]'}"><span class="h-1.5 w-1.5 rounded-full {node.connected ? 'status-dot bg-emerald-500' : 'bg-[var(--fg-subtle)]'}"></span>{node.connected ? 'Online' : 'Offline'}</span>
	</header>

	<div class="space-y-4">
		{#each metrics as metric (metric.label)}
			<div>
				<div class="mb-2 flex items-end justify-between gap-3"><div><p class="text-xs font-medium text-[var(--fg-muted)]">{metric.label}</p><p class="mt-0.5 text-[10px] text-[var(--fg-subtle)]">{metric.detail}</p></div><span class="text-lg font-semibold tracking-tight tabular-nums text-[var(--fg)]">{metric.value.toFixed(0)}<small class="ml-0.5 text-[10px] font-medium text-[var(--fg-subtle)]">%</small></span></div>
				<div class="h-1.5 overflow-hidden rounded-full bg-[var(--track)]"><div class="h-full rounded-full {metric.color} transition-all duration-700" style="width:{Math.min(100, Math.max(0, metric.value))}%"></div></div>
			</div>
		{/each}
	</div>

	{#if !node.connected && onShowCommand}
		<button type="button" onclick={() => onShowCommand?.(node)} class="mt-5 flex w-full items-center justify-center gap-2 rounded-xl border border-[var(--border)] bg-[var(--track)] px-3 py-2.5 text-[11px] font-semibold text-[var(--fg-muted)] transition hover:border-[var(--border-strong)] hover:text-[var(--fg)]">
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M8 9 4 12l4 3M16 9l4 3-4 3M14 5l-4 14" stroke-linecap="round" stroke-linejoin="round" /></svg>
			{awaitingInstallation ? 'Mostrar comando de instalación' : 'Generar comando de reinstalación'}
		</button>
	{/if}
</article>
