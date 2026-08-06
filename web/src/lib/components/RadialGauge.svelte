<script lang="ts">
	let { percent, label, sublabel, size = 92 }: { percent: number; label: string; sublabel: string; size?: number } =
		$props();

	const clamped = $derived(Math.min(100, Math.max(0, percent)));
	const radius = $derived(size / 2 - 8);
	const circumference = $derived(2 * Math.PI * radius);
	const offset = $derived(circumference - (clamped / 100) * circumference);
	const color = $derived(clamped > 90 ? '#f43f5e' : clamped > 70 ? '#f59e0b' : 'var(--accent)');
</script>

<div class="flex flex-col items-center gap-2">
	<div class="relative" style="width: {size}px; height: {size}px;">
		<svg width={size} height={size} class="-rotate-90">
			<circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke="rgba(255,255,255,0.15)" stroke-width="8" />
			<circle
				cx={size / 2}
				cy={size / 2}
				r={radius}
				fill="none"
				stroke={color}
				stroke-width="8"
				stroke-linecap="round"
				stroke-dasharray={circumference}
				stroke-dashoffset={offset}
				class="transition-all duration-500"
			/>
		</svg>
		<div class="absolute inset-0 grid place-items-center text-sm font-semibold text-white">
			{clamped.toFixed(0)}%
		</div>
	</div>
	<span class="text-xs font-medium text-white/80">{label}</span>
	<span class="text-[11px] text-white/50">{sublabel}</span>
</div>
