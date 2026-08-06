<script lang="ts">
	import '@xterm/xterm/css/xterm.css';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import TopBar from '$lib/components/TopBar.svelte';
	import { listNodes, terminalWsUrl, type Node } from '$lib/api';

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let status = $state<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle');

	const connectedNodes = $derived(nodes.filter((n) => n.connected));

	let container: HTMLDivElement;
	let term: Terminal | undefined;
	let fitAddon: FitAddon | undefined;
	let ws: WebSocket | undefined;

	function bytesToBase64(bytes: Uint8Array): string {
		let binary = '';
		for (const b of bytes) binary += String.fromCharCode(b);
		return btoa(binary);
	}

	function base64ToBytes(b64: string): Uint8Array {
		const binary = atob(b64);
		const bytes = new Uint8Array(binary.length);
		for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
		return bytes;
	}

	function connect(nodeId: string) {
		ws?.close();
		term?.reset();
		if (!term || !fitAddon) return;

		fitAddon.fit();
		status = 'connecting';
		const socket = new WebSocket(terminalWsUrl(nodeId, term.cols, term.rows));
		socket.onopen = () => (status = 'connected');
		socket.onclose = () => (status = 'closed');
		socket.onerror = () => (status = 'error');
		socket.onmessage = (event) => {
			try {
				const msg = JSON.parse(event.data);
				if (msg.type === 'output' && msg.data) {
					term?.write(base64ToBytes(msg.data));
				} else if (msg.type === 'closed') {
					term?.write('\r\n\x1b[90m[session closed]\x1b[0m\r\n');
					status = 'closed';
				}
			} catch {
				// ignore malformed frames
			}
		};
		ws = socket;
	}

	$effect(() => {
		term = new Terminal({
			convertEol: true,
			fontSize: 13,
			theme: { background: '#00000000' }
		});
		fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(container);
		fitAddon.fit();

		term.onData((data) => {
			if (ws?.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ type: 'input', data: bytesToBase64(new TextEncoder().encode(data)) }));
			}
		});

		const resizeObserver = new ResizeObserver(() => {
			if (!term || !fitAddon) return;
			fitAddon.fit();
			if (ws?.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
			}
		});
		resizeObserver.observe(container);

		return () => {
			resizeObserver.disconnect();
			ws?.close();
			term?.dispose();
		};
	});

	$effect(() => {
		(async () => {
			nodes = await listNodes();
			if (!selectedNodeId && connectedNodes.length > 0) {
				selectedNodeId = connectedNodes[0].id;
			}
		})();
	});

	$effect(() => {
		if (selectedNodeId && term) {
			connect(selectedNodeId);
		}
	});

	const statusLabel = $derived(
		{ idle: '', connecting: 'Connecting…', connected: 'Connected', closed: 'Disconnected', error: 'Connection error' }[
			status
		]
	);
</script>

<TopBar title="Terminal">
	{#if connectedNodes.length > 0}
		<select
			bind:value={selectedNodeId}
			class="rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
		>
			{#each connectedNodes as node (node.id)}
				<option value={node.id}>{node.name}</option>
			{/each}
		</select>
	{/if}
	{#if statusLabel}
		<span class="text-xs text-[var(--fg-subtle)]">{statusLabel}</span>
	{/if}
</TopBar>

<main class="flex flex-1 flex-col p-6">
	{#if connectedNodes.length === 0}
		<div class="grid flex-1 place-items-center rounded-2xl border border-dashed border-[var(--border)] text-center">
			<p class="text-[var(--fg-muted)]">No connected servers yet.</p>
		</div>
	{/if}
	<div
		bind:this={container}
		class="flex-1 overflow-hidden rounded-2xl border border-[var(--border)] bg-[#0b0d12] p-3"
		class:hidden={connectedNodes.length === 0}
	></div>
</main>
