<script lang="ts">
	let { label, percent, detail }: { label: string; percent: number; detail?: string } = $props();

	const clamped = $derived(Math.min(100, Math.max(0, percent)));
	const colorClass = $derived(
		clamped > 90 ? 'bg-rose-500' : clamped > 70 ? 'bg-amber-500' : 'bg-teal-500'
	);
</script>

<div class="flex flex-col gap-1.5">
	<div class="flex items-baseline justify-between text-sm">
		<span class="font-medium text-[var(--fg-muted)]">{label}</span>
		<span class="tabular-nums text-[var(--fg)]">{clamped.toFixed(0)}%</span>
	</div>
	<div class="h-2 w-full overflow-hidden rounded-full bg-[var(--track)]">
		<div class="h-full rounded-full {colorClass} transition-all duration-500" style="width: {clamped}%"></div>
	</div>
	{#if detail}
		<span class="text-xs text-[var(--fg-subtle)]">{detail}</span>
	{/if}
</div>
