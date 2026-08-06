<script lang="ts">
	import { dismiss, getToasts } from '$lib/toast.svelte';
</script>

<div class="pointer-events-none fixed inset-x-0 top-4 z-[100] flex flex-col items-center gap-2 px-4">
	{#each getToasts() as toast (toast.id)}
		<div
			role="status"
			class="pointer-events-auto flex max-w-sm items-center gap-3 rounded-xl border px-4 py-2.5 text-sm shadow-lg backdrop-blur
				{toast.kind === 'success'
				? 'border-[var(--accent)]/30 bg-[var(--surface)] text-[var(--fg)]'
				: 'border-rose-500/30 bg-[var(--surface)] text-[var(--fg)]'}"
		>
			<span
				class="h-2 w-2 shrink-0 rounded-full {toast.kind === 'success' ? 'bg-[var(--accent)]' : 'bg-rose-500'}"
			></span>
			<span class="flex-1">{toast.message}</span>
			<button
				onclick={() => dismiss(toast.id)}
				aria-label="Dismiss notification"
				class="text-[var(--fg-subtle)] hover:text-[var(--fg)]"
			>
				✕
			</button>
		</div>
	{/each}
</div>
