<script lang="ts">
	import { listNodes } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';

	let message = $state('Checking server connections…');

	async function loadMessage() {
		try {
			const nodes = await listNodes();
			message = nodes.some((node) => node.connected)
				? 'Storage management is coming soon'
				: 'No servers connected yet';
		} catch {
			message = 'Unable to check server connections';
		}
	}

	$effect(() => {
		void loadMessage();
	});
</script>

<TopBar title="Storage" />

<main class="flex-1 p-6">
	<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
		<p class="text-[var(--fg-muted)]">{message}</p>
	</div>
</main>
