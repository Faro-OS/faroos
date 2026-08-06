<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import NodeCard from '$lib/components/NodeCard.svelte';
	import { createPairing, listNodes, type Node, type PairingResult } from '$lib/api';

	let nodes = $state<Node[]>([]);
	let loadError = $state<string | null>(null);
	let showPairModal = $state(false);
	let newNodeName = $state('');
	let pairingResult = $state<PairingResult | null>(null);
	let pairing = $state(false);

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
		const interval = setInterval(refresh, 4000);
		return () => clearInterval(interval);
	});

	async function submitPairing(e: SubmitEvent) {
		e.preventDefault();
		if (!newNodeName.trim()) return;
		pairing = true;
		try {
			pairingResult = await createPairing(newNodeName.trim());
			refresh();
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to create pairing';
		} finally {
			pairing = false;
		}
	}

	function closeModal() {
		showPairModal = false;
		newNodeName = '';
		pairingResult = null;
	}
</script>

<TopBar title="Dashboard">
	<button
		onclick={() => (showPairModal = true)}
		class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)] transition-opacity hover:opacity-90"
	>
		+ Add server
	</button>
</TopBar>

<main class="flex-1 p-6">
	{#if loadError}
		<div class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-500">
			{loadError} — is the FaroOS server running?
		</div>
	{/if}

	{#if nodes.length === 0 && !loadError}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No servers paired yet.</p>
			<button
				onclick={() => (showPairModal = true)}
				class="mt-3 text-sm font-semibold text-[var(--accent)] hover:underline"
			>
				Pair your first server →
			</button>
		</div>
	{:else}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
			{#each nodes as node (node.id)}
				<NodeCard {node} />
			{/each}
		</div>
	{/if}
</main>

{#if showPairModal}
	<div class="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
		<div class="w-full max-w-md rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-xl">
			{#if !pairingResult}
				<h2 class="mb-1 text-lg font-semibold text-[var(--fg)]">Pair a new server</h2>
				<p class="mb-4 text-sm text-[var(--fg-subtle)]">
					Give it a name. You'll get credentials to run the FaroOS agent on that machine.
				</p>
				<form onsubmit={submitPairing} class="flex flex-col gap-3">
					<input
						bind:value={newNodeName}
						placeholder="e.g. home-server"
						class="rounded-xl border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
					/>
					<div class="flex justify-end gap-2">
						<button type="button" onclick={closeModal} class="rounded-xl px-4 py-2 text-sm text-[var(--fg-muted)] hover:text-[var(--fg)]">
							Cancel
						</button>
						<button
							type="submit"
							disabled={pairing}
							class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)] disabled:opacity-50"
						>
							{pairing ? 'Pairing…' : 'Generate credentials'}
						</button>
					</div>
				</form>
			{:else}
				<h2 class="mb-1 text-lg font-semibold text-[var(--fg)]">Run this on {pairingResult.name}</h2>
				<p class="mb-3 text-sm text-[var(--fg-subtle)]">
					This token is shown once. Copy it into the agent's environment before closing this dialog.
				</p>
				<pre class="overflow-x-auto rounded-xl bg-[var(--bg)] p-4 text-xs text-[var(--fg)]"><code
					>FAROOS_SERVER=ws://&lt;this-server&gt;:8090/api/agent/connect
FAROOS_NODE_ID={pairingResult.id}
FAROOS_TOKEN={pairingResult.token}
./faroos-agent</code
				></pre>
				<div class="mt-4 flex justify-end">
					<button onclick={closeModal} class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)]">
						Done
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
