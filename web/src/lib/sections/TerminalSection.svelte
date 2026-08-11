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

	// Keep the shell neutral and predictable. The default renderer is used
	// intentionally: WebGL is faster on paper, but a context loss could leave
	// an otherwise healthy remote shell rendering as an empty black panel.
	const terminalTheme = {
		background: '#0c0d0f',
		foreground: '#f0f0f2',
		cursor: '#f0f0f2',
		cursorAccent: '#0c0d0f',
		selectionBackground: '#ffffff2b',
		black: '#1a1d24',
		red: '#f43f5e',
		green: '#50c878',
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
		socket.onopen = () => {
			status = 'connected';
			term?.focus();
		};
		socket.onclose = () => {
			status = 'closed';
			term?.write('\r\n\x1b[90m[connection closed — use Reconnect to start a new session]\x1b[0m\r\n');
		};
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
		term.writeln('\x1b[90mFaroOS secure terminal · establishing session…\x1b[0m');

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
			class="control rounded-xl px-3 text-sm text-[var(--fg)] outline-none"
		>
			{#each connectedNodes as node (node.id)}
				<option value={node.id}>{node.name}</option>
			{/each}
		</select>
	{/if}
	{#if statusLabel}
		<span class="flex items-center gap-1.5 rounded-full bg-[var(--track)] px-2.5 py-1 text-[10px] font-semibold text-[var(--fg-subtle)]"><span class="h-1.5 w-1.5 rounded-full {status === 'connected' ? 'bg-emerald-500' : 'bg-[var(--fg-subtle)]'}"></span>{statusLabel}</span>
	{/if}
	{#if selectedNodeId}<button type="button" onclick={() => connect(selectedNodeId!)} class="control rounded-xl px-3 py-2 text-xs font-semibold text-[var(--fg-muted)]">Reconnect</button>{/if}
</TopBar>

<main class="section-enter mx-auto flex min-h-[calc(100dvh-78px)] w-full max-w-[1480px] flex-col p-4 pb-32 sm:p-7 sm:pb-32 lg:p-10 lg:pb-32">
	{#if connectedNodes.length === 0}
		<div class="surface-card grid flex-1 place-items-center rounded-[24px] border-dashed text-center">
			<p class="text-[var(--fg-muted)]">No connected servers yet.</p>
		</div>
	{/if}
	<div
		bind:this={container}
		class="min-h-[520px] flex-1 overflow-hidden rounded-[20px] border border-white/8 bg-[#0c0d0f] p-4 shadow-[var(--shadow-md)]"
		class:hidden={connectedNodes.length === 0}
	></div>
</main>
