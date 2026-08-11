<script lang="ts">
	import { untrack } from 'svelte';
	import AppIcon from './AppIcon.svelte';
	import {
		containerAction,
		deployApp,
		inspectPort,
		type AppEnvVar,
		type AppPort,
		type CatalogApp
	} from '$lib/api';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	let { app, nodeId, onClose, onDeployed }: {
		app: CatalogApp;
		nodeId: string;
		onClose: () => void;
		onDeployed: () => void;
	} = $props();

	type PortRowStatus = 'idle' | 'checking' | 'free' | 'busy-own' | 'busy-other';

	let ports = $state<AppPort[]>(untrack(() => (app.ports ?? []).map((p) => ({ ...p }))));
	let env = $state<AppEnvVar[]>(untrack(() => app.env?.map((e) => ({ ...e, default: e.default })) ?? []));
	let argumentsValue = $state(untrack(() => app.arguments ?? ''));
	let argumentsRevealed = $state(false);
	let portStatus = $state<PortRowStatus[]>(untrack(() => (app.ports ?? []).map(() => 'idle')));
	let portContainer = $state<(string | undefined)[]>(untrack(() => (app.ports ?? []).map(() => undefined)));
	let portContainerId = $state<(string | undefined)[]>(untrack(() => (app.ports ?? []).map(() => undefined)));
	let freeing = $state<number | null>(null);
	let revealed = $state(new Set<number>());
	let deploying = $state(false);
	let error = $state<string | null>(null);

	let checkTimers: ReturnType<typeof setTimeout>[] = [];

	function isSecret(key: string): boolean {
		return /PASSWORD|SECRET|TOKEN|_KEY$|^KEY|APIKEY/i.test(key);
	}

	async function checkPort(index: number) {
		const port = ports[index]?.host;
		if (!port || port < 1 || port > 65535) {
			portStatus[index] = 'idle';
			return;
		}
		portStatus[index] = 'checking';
		try {
			const status = await inspectPort(nodeId, port);
			if (!status.inUse) {
				portStatus[index] = 'free';
				portContainer[index] = undefined;
				portContainerId[index] = undefined;
			} else if (status.ownApp) {
				portStatus[index] = 'busy-own';
				portContainer[index] = status.containerName;
				portContainerId[index] = status.containerId;
			} else {
				portStatus[index] = 'busy-other';
				portContainer[index] = undefined;
				portContainerId[index] = undefined;
			}
		} catch {
			portStatus[index] = 'idle';
		}
	}

	function schedulePortCheck(index: number) {
		clearTimeout(checkTimers[index]);
		checkTimers[index] = setTimeout(() => checkPort(index), 400);
	}

	$effect(() => {
		ports.forEach((_, i) => checkPort(i));
		return () => checkTimers.forEach((t) => clearTimeout(t));
	});

	async function freeAndUsePort(index: number) {
		const id = portContainerId[index];
		if (!id) return;
		freeing = index;
		try {
			await containerAction(nodeId, id, 'stop');
			toastSuccess(`${portContainer[index]} stopped`);
			await checkPort(index);
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Failed to stop container');
		} finally {
			freeing = null;
		}
	}

	function toggleReveal(index: number) {
		const next = new Set(revealed);
		if (next.has(index)) next.delete(index);
		else next.add(index);
		revealed = next;
	}

	function argumentsContainSecret(): boolean {
		return /token|password|secret|api.?key/i.test(argumentsValue);
	}

	function hasUnresolvedPlaceholder(): boolean {
		return /YOUR_[A-Z0-9_]+(?:_HERE)?|CHANGE[_ -]?ME|<[^>]*(?:TOKEN|PASSWORD|SECRET|KEY)[^>]*>/i.test(argumentsValue);
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;

		if (portStatus.some((s) => s === 'busy-other' || s === 'busy-own')) {
			error = 'Resolve the port conflicts below before installing.';
			return;
		}
		if (hasUnresolvedPlaceholder()) {
			error = 'Replace the credential placeholder in Container arguments before installing.';
			return;
		}

		deploying = true;
		try {
			await deployApp(nodeId, app.id, { ports, env, arguments: argumentsValue });
			toastSuccess(`${app.name} deployed`);
			onDeployed();
		} catch (err) {
			const message = err instanceof Error ? err.message : `Failed to deploy ${app.name}`;
			error = message;
			toastError(message);
		} finally {
			deploying = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="install-layer fixed inset-0 z-[70] grid place-items-center p-4">
	<button type="button" class="install-scrim absolute inset-0" aria-label="Close installer" onclick={onClose}></button>
	<div class="install-panel glass relative flex max-h-[85vh] w-full max-w-lg flex-col rounded-[28px]">
		<div class="flex items-center gap-3 border-b border-[var(--border)] p-5">
			<AppIcon name={app.name} icon={app.icon} size={48} />
			<div class="min-w-0 flex-1">
				<h2 class="truncate font-semibold text-[var(--fg)]">{app.name}</h2>
				<p class="text-xs text-[var(--fg-subtle)]">{app.category ?? 'Other'}</p>
			</div>
			<button onclick={onClose} aria-label="Close" class="control grid h-9 w-9 min-h-0 shrink-0 place-items-center rounded-xl text-[var(--fg-subtle)] hover:text-[var(--fg)]"><svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 6 12 12M18 6 6 18" /></svg></button>
		</div>

		<form id="install-app-form" onsubmit={handleSubmit} class="flex-1 overflow-y-auto p-5">
			{#if ports.length > 0}
				<section class="mb-6">
					<h3 class="mb-3 text-sm font-semibold text-[var(--fg)]">Ports</h3>
					<div class="flex flex-col gap-3">
						{#each ports as port, i (i)}
							<div class="rounded-xl border border-[var(--border)] p-3">
								<div class="flex items-center gap-3">
									<span class="w-24 shrink-0 text-xs text-[var(--fg-subtle)]">Container {port.container}/{port.protocol}</span>
									<span class="text-[var(--fg-subtle)]">→</span>
									<input
										type="number"
										min="1"
										max="65535"
										bind:value={port.host}
										oninput={() => schedulePortCheck(i)}
										class="w-28 rounded-lg border border-[var(--border)] bg-[var(--bg)] px-2.5 py-1.5 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
									/>
									<span class="flex-1"></span>
									{#if portStatus[i] === 'checking'}
										<span class="text-xs text-[var(--fg-subtle)]">Checking…</span>
									{:else if portStatus[i] === 'free'}
										<span class="flex items-center gap-1 text-xs font-medium text-[var(--accent)]">
											<span class="h-1.5 w-1.5 rounded-full bg-[var(--accent)]"></span> Available
										</span>
									{:else if portStatus[i] === 'busy-own' || portStatus[i] === 'busy-other'}
										<span class="flex items-center gap-1 text-xs font-medium text-rose-500">
											<span class="h-1.5 w-1.5 rounded-full bg-rose-500"></span> In use
										</span>
									{/if}
								</div>
								{#if portStatus[i] === 'busy-own'}
									<div class="mt-2 flex items-center justify-between gap-2 rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-500">
										<span>Used by FaroOS app <strong>{portContainer[i]}</strong></span>
										<button
											type="button"
											onclick={() => freeAndUsePort(i)}
											disabled={freeing === i}
											class="shrink-0 rounded-lg bg-rose-500 px-2.5 py-1 font-semibold text-white disabled:opacity-50"
										>
											{freeing === i ? 'Stopping…' : 'Stop & free port'}
										</button>
									</div>
								{:else if portStatus[i] === 'busy-other'}
									<p class="mt-2 rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-500">
										In use by another service on this server — pick a different port instead.
									</p>
								{/if}
							</div>
						{/each}
					</div>
				</section>
			{/if}

			{#if env.length > 0}
				<section class="mb-2">
					<h3 class="mb-3 text-sm font-semibold text-[var(--fg)]">Environment & credentials</h3>
					<div class="flex flex-col gap-3">
						{#each env as variable, i (variable.key)}
							<label class="flex flex-col gap-1">
								<span class="font-mono text-xs font-semibold text-[var(--fg)]">{variable.key}</span>
								{#if variable.description}
									<span class="text-xs text-[var(--fg-subtle)]">{variable.description}</span>
								{/if}
								<div class="relative">
									<input
										type={isSecret(variable.key) && !revealed.has(i) ? 'password' : 'text'}
										bind:value={variable.default}
										class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg)] px-3 py-2 pr-9 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
									/>
									{#if isSecret(variable.key)}
										<button
											type="button"
											onclick={() => toggleReveal(i)}
											class="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--fg-subtle)] hover:text-[var(--fg)]"
											aria-label={revealed.has(i) ? 'Hide value' : 'Show value'}
										>
											{#if revealed.has(i)}
												<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12s3.5-7 9-7 9 7 9 7-3.5 7-9 7-9-7-9-7Z" /><circle cx="12" cy="12" r="3" /></svg>
											{:else}
												<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 3l18 18M10.6 10.6a3 3 0 0 0 4.24 4.24M9.88 4.24A9.5 9.5 0 0 1 12 4c5.5 0 9 7 9 7a13.4 13.4 0 0 1-3.14 3.86M6.1 6.1A13.6 13.6 0 0 0 3 11s3.5 7 9 7a9.5 9.5 0 0 0 3.9-.84" /></svg>
											{/if}
										</button>
									{/if}
								</div>
							</label>
						{/each}
					</div>
				</section>
			{/if}

			{#if app.arguments}
				<section class="mb-2 mt-6">
					<h3 class="mb-1 text-sm font-semibold text-[var(--fg)]">Container arguments</h3>
					<p class="mb-3 text-xs leading-5 text-[var(--fg-subtle)]">Passed directly to the app when it starts. Replace any credential placeholder supplied by the catalog.</p>
					<div class="relative">
						<input
							type={argumentsContainSecret() && !argumentsRevealed ? 'password' : 'text'}
							bind:value={argumentsValue}
							spellcheck="false"
							class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg)] px-3 py-2 pr-10 font-mono text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
						/>
						{#if argumentsContainSecret()}
							<button
								type="button"
								onclick={() => (argumentsRevealed = !argumentsRevealed)}
								class="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--fg-subtle)] hover:text-[var(--fg)]"
								aria-label={argumentsRevealed ? 'Hide arguments' : 'Show arguments'}
							>
								{argumentsRevealed ? '◉' : '○'}
							</button>
						{/if}
					</div>
					{#if hasUnresolvedPlaceholder()}
						<p class="mt-2 text-xs font-medium text-amber-600">A required credential still needs to be replaced.</p>
					{/if}
				</section>
			{/if}

			{#if ports.length === 0 && env.length === 0 && !app.arguments}
				<p class="py-8 text-center text-sm text-[var(--fg-subtle)]">This app has no configurable ports or environment variables — it'll deploy with its defaults.</p>
			{/if}

			{#if error}
				<p class="mt-2 text-sm text-rose-500">{error}</p>
			{/if}
		</form>

		<div class="flex justify-end gap-2 border-t border-[var(--border)] p-4">
			<button type="button" onclick={onClose} class="rounded-xl px-4 py-2 text-sm font-semibold text-[var(--fg-muted)] hover:text-[var(--fg)]">
				Cancel
			</button>
			<button
				type="submit"
				form="install-app-form"
				disabled={deploying || hasUnresolvedPlaceholder() || portStatus.some((s) => s === 'busy-own' || s === 'busy-other')}
				class="primary-control rounded-xl px-5 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
			>
				{deploying ? 'Installing…' : 'Install'}
			</button>
		</div>
	</div>
</div>

<style>
	.install-layer { perspective: 1000px; }
	.install-scrim { border: 0; background: color-mix(in srgb,#07080b 42%,transparent); animation: installer-scrim 180ms ease-out both; backdrop-filter: blur(9px) saturate(.85); -webkit-backdrop-filter: blur(9px) saturate(.85); }
	.install-panel { transform-origin: center 25%; animation: installer-in 340ms var(--motion-settle) both; }
	@keyframes installer-scrim { from { opacity:0; } to { opacity:1; } }
	@keyframes installer-in { from { opacity:0; transform:translateY(12px) scale(.97); filter:blur(7px); } to { opacity:1; transform:translateY(0) scale(1); filter:blur(0); } }
	@media (prefers-reduced-motion: reduce) { .install-scrim,.install-panel { animation:none; } }
	@media (prefers-reduced-transparency: reduce) { .install-scrim { backdrop-filter:none; -webkit-backdrop-filter:none; } }
</style>
