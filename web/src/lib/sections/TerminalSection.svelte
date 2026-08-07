<script lang="ts">
	import '@xterm/xterm/css/xterm.css';
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import { WebglAddon } from '@xterm/addon-webgl';
	import TopBar from '$lib/components/TopBar.svelte';
	import { listNodes, terminalWsUrl, type Node } from '$lib/api';

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let status = $state<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle');
	let gpuAccelerated = $state(false);

	const connectedNodes = $derived(nodes.filter((n) => n.connected));

	let container: HTMLDivElement;
	let term: Terminal | undefined;
	let fitAddon: FitAddon | undefined;
	let webglAddon: WebglAddon | undefined;
	let ws: WebSocket | undefined;

	// A Rio-inspired palette: deep near-black background, a vivid cyan
	// accent, and saturated-but-not-garish ANSI colors.
	const terminalTheme = {
		background: '#0b0d12',
		foreground: '#e8e9ec',
		cursor: '#2dd4bf',
		cursorAccent: '#0b0d12',
		selectionBackground: '#2dd4bf40',
		black: '#1a1d24',
		red: '#f43f5e',
		green: '#22c55e',
		yellow: '#eab308',
		blue: '#3b82f6',
		magenta: '#d946ef',
		cyan: '#2dd4bf',
		white: '#e8e9ec',
		brightBlack: '#4b5160',
		brightRed: '#fb7185',
		brightGreen: '#4ade80',
		brightYellow: '#facc15',
		brightBlue: '#60a5fa',
		brightMagenta: '#e879f9',
		brightCyan: '#5eead4',
		brightWhite: '#ffffff'
	};

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
			fontFamily: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
			cursorBlink: true,
			cursorStyle: 'bar',
			theme: terminalTheme
		});
		fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(container);
		fitAddon.fit();

		// GPU-accelerated rendering when available (not guaranteed in every
		// browser/context — e.g. some sandboxed embeds disable WebGL2) —
		// falls back to xterm's default canvas renderer if it throws.
		try {
			webglAddon = new WebglAddon();
			webglAddon.onContextLoss(() => webglAddon?.dispose());
			term.loadAddon(webglAddon);
			gpuAccelerated = true;
		} catch {
			gpuAccelerated = false;
		}

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
			webglAddon?.dispose();
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
	{#if gpuAccelerated}
		<span class="flex items-center gap-1 text-xs text-[var(--accent)]" title="Rendered on the GPU via WebGL">
			<svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M13 2 3 14h7l-1 8 11-14h-7l1-6Z" stroke-linecap="round" stroke-linejoin="round" />
			</svg>
			GPU
		</span>
	{/if}
</TopBar>

<main class="flex flex-1 flex-col p-6 pb-28">
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
