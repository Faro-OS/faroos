<script lang="ts">
	import { createPairing, listNodes, relayStatus, renewPairing, type Node, type PairingResult } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import NodeCard from '$lib/components/NodeCard.svelte';
	import { formatMemory, formatStorage, totalStorageBytes } from '$lib/format';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	let nodes = $state<Node[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let showPairingModal = $state(false);
	let pairingResult = $state<PairingResult | null>(null);
	let serverName = $state('');
	let panelAddress = $state('');
	let pairing = $state(false);
	let reopeningCommand = $state(false);
	let relayEnabled = $state(false);
	let p2pEnabled = $state(false);
	const online = $derived(nodes.filter((node) => node.connected));
	const totalMemory = $derived(online.reduce((sum, node) => sum + node.stats.memTotalBytes, 0));
	const totalStorage = $derived(online.reduce((sum, node) => sum + totalStorageBytes(node.stats), 0));
	const averageCpu = $derived(online.length ? online.reduce((sum, node) => sum + node.stats.cpuPercent, 0) / online.length : 0);
	const pairingConnected = $derived(
		Boolean(pairingResult && nodes.find((node) => node.id === pairingResult?.id)?.connected)
	);

	async function refresh() {
		try {
			nodes = await listNodes();
			loadError = null;
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load servers';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void refresh();
		void relayStatus().then((status) => {
			relayEnabled = status.enabled;
			p2pEnabled = status.p2p;
		}).catch(() => {});
		const interval = setInterval(refresh, 4000);
		return () => clearInterval(interval);
	});

	function openAddServer() {
		pairingResult = null;
		serverName = '';
		panelAddress = typeof window !== 'undefined' ? window.location.origin : '';
		reopeningCommand = false;
		showPairingModal = true;
	}

	function closePairing() {
		showPairingModal = false;
		pairingResult = null;
		serverName = '';
		pairing = false;
		reopeningCommand = false;
	}

	async function addServer(event: SubmitEvent) {
		event.preventDefault();
		if (!serverName.trim() || pairing) return;
		if (!relayEnabled && !resolvedPanelAddress()) {
			toastError('Introduce una dirección pública HTTP o HTTPS válida para el panel');
			return;
		}
		pairing = true;
		try {
			pairingResult = await createPairing(serverName.trim());
			await refresh();
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'No se pudo añadir el servidor');
		} finally {
			pairing = false;
		}
	}

	async function showCommandAgain(node: Node) {
		const hasConnectedBefore = new Date(node.lastSeen).getTime() > 0;
		if (hasConnectedBefore && !window.confirm('Se invalidará el comando anterior y el agente deberá reinstalarse. ¿Continuar?')) return;
		pairingResult = null;
		serverName = node.name;
		panelAddress = typeof window !== 'undefined' ? window.location.origin : '';
		reopeningCommand = true;
		pairing = true;
		showPairingModal = true;
		try {
			pairingResult = await renewPairing(node.id);
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'No se pudo generar otro comando');
			closePairing();
		} finally {
			pairing = false;
		}
	}

	function installCommand(result: PairingResult): string {
		const panel = result.panelUrl || resolvedPanelAddress();
		if (!panel) return '';
		const parsed = new URL(panel);
		const wsProtocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
		const basePath = parsed.pathname.replace(/\/+$/, '');
		const websocket = `${wsProtocol}//${parsed.host}${basePath}/api/agent/connect`;
		return `curl -fsSL ${shellQuote(`${panel}/install/agent.sh`)} | sudo bash -s -- --panel ${shellQuote(panel)} --server ${shellQuote(websocket)} --node ${shellQuote(result.id)} --token ${shellQuote(result.token)}`;
	}

	function resolvedPanelAddress(): string | null {
		const input = panelAddress.trim();
		if (!input) return null;
		try {
			const parsed = new URL(input.includes('://') ? input : `https://${input}`);
			if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
			return parsed.origin + parsed.pathname.replace(/\/+$/, '');
		} catch {
			return null;
		}
	}

	function shellQuote(value: string): string {
		return `'${value.replaceAll("'", `'"'"'`)}'`;
	}

	async function copyCommand() {
		if (!pairingResult) return;
		const command = installCommand(pairingResult);
		try {
			let copied = false;
			if (navigator.clipboard?.writeText) {
				try {
					await navigator.clipboard.writeText(command);
					copied = true;
				} catch {
					// Fall through for LAN panels served over HTTP.
				}
			}
			if (!copied) {
				const input = document.createElement('textarea');
				input.value = command;
				input.style.position = 'fixed';
				input.style.opacity = '0';
				document.body.appendChild(input);
				input.select();
				copied = document.execCommand('copy');
				input.remove();
			}
			if (!copied) throw new Error('Clipboard unavailable');
			toastSuccess('Comando copiado');
		} catch {
			toastError('No se pudo copiar; selecciona el comando manualmente');
		}
	}
</script>

<TopBar title="Servers">
	<button type="button" onclick={openAddServer} class="primary-control flex h-10 items-center gap-2 rounded-xl px-3.5 text-xs font-semibold"><span class="text-base leading-none">＋</span><span class="hidden sm:inline">Añadir servidor</span></button>
</TopBar>

<main class="section-enter mx-auto w-full max-w-[1480px] p-4 pb-32 sm:p-7 sm:pb-32 lg:p-10 lg:pb-32">
	{#if loadError}<div class="mb-5 flex items-center gap-2 rounded-2xl border border-rose-500/15 bg-rose-500/8 px-4 py-3 text-sm text-rose-500"><span class="h-2 w-2 rounded-full bg-rose-500"></span>{loadError} — is the FaroOS server running?</div>{/if}

	<div class="mb-7 grid grid-cols-2 gap-3 lg:grid-cols-4">
		{#each [
			{ label: 'Connected', value: `${online.length} / ${nodes.length}`, detail: 'Servers online', tint: 'text-[var(--fg)]' },
			{ label: 'Average load', value: `${averageCpu.toFixed(0)}%`, detail: 'Across the fleet', tint: 'text-[var(--fg)]' },
			{ label: 'Total memory', value: formatMemory(totalMemory), detail: 'Available online', tint: 'text-[var(--fg)]' },
			{ label: 'Total storage', value: formatStorage(totalStorage), detail: 'All detected devices', tint: 'text-[var(--fg)]' }
		] as stat (stat.label)}
			<div class="surface-card rounded-[20px] p-4 sm:p-5"><p class="eyebrow mb-3">{stat.label}</p><p class="text-2xl font-semibold tracking-[-0.045em] {stat.tint}">{stat.value}</p><p class="mt-1 text-[11px] text-[var(--fg-subtle)]">{stat.detail}</p></div>
		{/each}
	</div>

	{#if loading}
		<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{#each [1, 2, 3] as item (item)}<div class="surface-card h-80 animate-pulse rounded-[24px]"></div>{/each}</div>
	{:else if nodes.length === 0 && !loadError}
		<div class="surface-card grid min-h-80 place-items-center rounded-[24px] border-dashed text-center"><div><span class="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-[var(--track)] text-[var(--fg-subtle)]">+</span><p class="font-medium text-[var(--fg)]">Todavía no hay servidores</p><p class="mt-1 text-sm text-[var(--fg-subtle)]">Añade el primero con un único comando.</p><button type="button" onclick={openAddServer} class="primary-control mt-5 rounded-xl px-4 py-2.5 text-xs font-semibold">Añadir servidor</button></div></div>
	{:else}
		<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{#each nodes as node (node.id)}<NodeCard {node} onShowCommand={showCommandAgain} />{/each}</div>
	{/if}
</main>

{#if showPairingModal}
	<div class="pairing-layer fixed inset-0 z-80 grid place-items-center p-4 sm:p-6">
		<button type="button" class="pairing-scrim absolute inset-0 border-0" aria-label="Cerrar" onclick={closePairing}></button>
		<div class="pairing-panel premium-card relative max-h-[calc(100dvh-2rem)] w-full max-w-3xl overflow-y-auto rounded-[26px] p-5 sm:p-7" role="dialog" aria-modal="true" aria-labelledby="pairing-title">
			{#if !pairingResult && reopeningCommand}
				<div class="grid min-h-52 place-items-center text-center"><div><span class="mx-auto mb-4 block h-8 w-8 animate-spin rounded-full border-2 border-[var(--border)] border-t-[var(--fg)]"></span><h2 id="pairing-title" class="text-xl font-semibold text-[var(--fg)]">Generando otro comando…</h2><p class="mt-2 text-sm text-[var(--fg-subtle)]">La credencial anterior quedará invalidada.</p></div></div>
			{:else if !pairingResult}
				<p class="eyebrow mb-2">Añadir servidor</p><h2 id="pairing-title" class="text-2xl font-semibold tracking-tight text-[var(--fg)]">Genera el comando de instalación</h2><p class="mt-2 text-sm text-[var(--fg-subtle)]">Incluye instalación, configuración y autenticación automática.</p>
				<form onsubmit={addServer} class="mt-6"><label for="new-server-name" class="mb-2 block text-xs font-semibold text-[var(--fg-muted)]">Nombre del servidor</label><input id="new-server-name" bind:value={serverName} placeholder="Servidor principal" class="h-12 w-full rounded-xl border border-[var(--border)] bg-[var(--track)] px-3.5 text-sm text-[var(--fg)] outline-none focus:border-[var(--border-strong)]" />{#if relayEnabled}<div class="mt-4 rounded-xl border border-emerald-500/20 bg-emerald-500/6 p-3.5"><strong class="text-xs text-[var(--fg)]">{p2pEnabled ? 'Conexión directa P2P activa' : 'FaroOS Relay activo'}</strong><p class="mt-1 text-[11px] text-[var(--fg-subtle)]">{p2pEnabled ? 'FaroOS solo coordina el encuentro; después los servidores se conectan directamente cuando la red lo permite.' : 'El comando funcionará desde cualquier red, sin VPN ni puertos abiertos.'}</p></div>{:else}<label for="panel-address" class="mb-2 mt-4 block text-xs font-semibold text-[var(--fg-muted)]">Dirección accesible desde ese servidor</label><input id="panel-address" type="url" bind:value={panelAddress} placeholder="https://panel.tudominio.com" class="h-12 w-full rounded-xl border border-[var(--border)] bg-[var(--track)] px-3.5 text-sm text-[var(--fg)] outline-none focus:border-[var(--border-strong)]" /><p class="mt-2 text-[11px] text-[var(--fg-subtle)]">Si está en otra red, usa un dominio HTTPS público o la dirección de tu VPN, no una IP privada local.</p>{/if}<div class="mt-5 flex justify-end gap-2"><button type="button" onclick={closePairing} class="rounded-xl px-4 py-2.5 text-xs font-semibold text-[var(--fg-muted)]">Cancelar</button><button type="submit" class="primary-control rounded-xl px-4 py-2.5 text-xs font-semibold" disabled={pairing}>{pairing ? 'Generando…' : 'Generar comando'}</button></div></form>
			{:else}
				<p class="eyebrow mb-2">Un comando · Linux</p><h2 id="pairing-title" class="text-2xl font-semibold tracking-tight text-[var(--fg)]">Instala y autentica {pairingResult.name}</h2><p class="mt-2 text-sm leading-relaxed text-[var(--fg-subtle)]">Copia esta única línea en una terminal del servidor. Instala Docker si hace falta, instala el agente, guarda la autenticación e inicia el servicio.</p>
				<div class="mt-6 grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-3 rounded-2xl bg-[#171b22] p-3 sm:grid-cols-[auto_minmax(0,1fr)_auto]"><span class="font-mono text-sm font-bold text-emerald-400">$</span><code class="min-w-0 overflow-x-auto whitespace-nowrap py-2 font-mono text-[11px] text-slate-200 select-all">{installCommand(pairingResult)}</code><button type="button" onclick={copyCommand} class="primary-control col-span-2 rounded-xl px-4 py-2.5 text-xs font-semibold sm:col-span-1">Copiar comando</button></div>
				<div class="mt-4 flex flex-wrap gap-2 text-[10px] font-semibold text-[var(--fg-muted)]"><span class="rounded-lg bg-[var(--track)] px-2.5 py-1.5">✓ Docker</span><span class="rounded-lg bg-[var(--track)] px-2.5 py-1.5">✓ Agente</span><span class="rounded-lg bg-[var(--track)] px-2.5 py-1.5">✓ Autenticación</span><span class="rounded-lg bg-[var(--track)] px-2.5 py-1.5">✓ Servicio automático</span></div>
				<div class="mt-5 flex items-center gap-3 rounded-xl border p-3.5 {pairingConnected ? 'border-emerald-500/20 bg-emerald-500/6' : 'border-[var(--border)] bg-[var(--track)]'}" aria-live="polite"><i class="h-2.5 w-2.5 shrink-0 rounded-full {pairingConnected ? 'bg-emerald-500' : 'animate-pulse bg-[var(--fg-subtle)]'}"></i><span class="flex flex-col"><strong class="text-xs text-[var(--fg)]">{pairingConnected ? 'Servidor autenticado y conectado' : 'Esperando conexión…'}</strong><small class="mt-0.5 text-[10px] text-[var(--fg-subtle)]">{pairingConnected ? 'Ya puedes administrarlo desde FaroOS.' : 'Esta pantalla lo detectará automáticamente.'}</small></span></div>
				<div class="mt-5 flex justify-between gap-2"><button type="button" onclick={openAddServer} class="rounded-xl px-3 py-2.5 text-xs font-semibold text-[var(--fg-muted)]">＋ Añadir otro servidor</button><button type="button" onclick={closePairing} class="rounded-xl px-4 py-2.5 text-xs font-semibold text-[var(--fg)]">{pairingConnected ? 'Terminar' : 'Cerrar'}</button></div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.pairing-layer { perspective: 1000px; }
	.pairing-scrim { background: color-mix(in srgb,#07080b 38%,transparent); animation: pairing-scrim-in 180ms ease-out both; backdrop-filter: blur(9px) saturate(.85); -webkit-backdrop-filter: blur(9px) saturate(.85); }
	.pairing-panel { transform-origin: center 25%; animation: pairing-panel-in 340ms var(--motion-settle) both; }
	@keyframes pairing-scrim-in { from { opacity:0; } to { opacity:1; } }
	@keyframes pairing-panel-in { from { opacity:0; transform:translateY(11px) scale(.97); filter:blur(7px); } to { opacity:1; transform:translateY(0) scale(1); filter:blur(0); } }
	@media (prefers-reduced-motion: reduce) { .pairing-scrim,.pairing-panel { animation:none; } }
	@media (prefers-reduced-transparency: reduce) { .pairing-scrim { backdrop-filter:none; -webkit-backdrop-filter:none; }.pairing-panel { background:var(--surface-solid); backdrop-filter:none; -webkit-backdrop-filter:none; } }
</style>
