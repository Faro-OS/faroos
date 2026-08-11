<script lang="ts">
	import { dismiss, getToasts } from '$lib/toast.svelte';
</script>

<div class="pointer-events-none fixed inset-x-0 top-4 z-[100] flex flex-col items-center gap-2 px-4" aria-live="polite" aria-atomic="true">
	{#each getToasts() as toast (toast.id)}
		<div
			role="status"
			class="toast-item glass pointer-events-auto flex max-w-sm items-center gap-3 rounded-2xl px-4 py-2.5 text-sm text-[var(--fg)]"
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

<style>
	.toast-item {
		transform-origin: center top;
		animation: toast-in 320ms var(--motion-settle) both;
	}

	@keyframes toast-in {
		from { opacity: 0; transform: translateY(-12px) scale(.96); filter: blur(5px); }
		to { opacity: 1; transform: translateY(0) scale(1); filter: blur(0); }
	}

	@media (prefers-reduced-motion: reduce) {
		.toast-item { animation: none; }
	}
</style>
