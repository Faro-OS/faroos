<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import { formatBytes } from '$lib/format';
	import {
		deleteFile,
		fileDownloadUrl,
		listFiles,
		listNodes,
		uploadFile,
		type FileEntry,
		type Node
	} from '$lib/api';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let currentPath = $state('/');
	let entries = $state<FileEntry[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let uploading = $state(false);
	let fileInput = $state<HTMLInputElement>();

	const connectedNodes = $derived(nodes.filter((n) => n.connected));
	const breadcrumbs = $derived.by(() => {
		const parts = currentPath.split('/').filter(Boolean);
		const crumbs: { label: string; path: string }[] = [{ label: 'root', path: '/' }];
		let acc = '';
		for (const part of parts) {
			acc += '/' + part;
			crumbs.push({ label: part, path: acc });
		}
		return crumbs;
	});

	async function loadNodes() {
		nodes = await listNodes();
		if (!selectedNodeId && connectedNodes.length > 0) {
			selectedNodeId = connectedNodes[0].id;
		}
	}

	async function loadEntries() {
		if (!selectedNodeId) {
			entries = [];
			loading = false;
			return;
		}
		loading = true;
		try {
			entries = await listFiles(selectedNodeId, currentPath);
			error = null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to list files';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		(async () => {
			await loadNodes();
			await loadEntries();
		})();
	});

	$effect(() => {
		if (selectedNodeId !== null) {
			currentPath = '/';
		}
	});

	$effect(() => {
		if (selectedNodeId) {
			void loadEntries();
		}
	});

	function joinPath(base: string, name: string): string {
		return base === '/' ? `/${name}` : `${base}/${name}`;
	}

	function open(entry: FileEntry) {
		if (entry.isDir) {
			currentPath = joinPath(currentPath, entry.name);
			void loadEntries();
		}
	}

	async function remove(entry: FileEntry) {
		if (!selectedNodeId) return;
		if (!confirm(`Delete ${entry.name}? This can't be undone.`)) return;
		try {
			await deleteFile(selectedNodeId, joinPath(currentPath, entry.name));
			await loadEntries();
			toastSuccess(`${entry.name} deleted`);
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to delete';
			error = message;
			toastError(message);
		}
	}

	async function handleUpload(e: Event) {
		if (!selectedNodeId) return;
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		uploading = true;
		try {
			await uploadFile(selectedNodeId, joinPath(currentPath, file.name), file);
			await loadEntries();
			toastSuccess(`${file.name} uploaded`);
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Upload failed';
			error = message;
			toastError(message);
		} finally {
			uploading = false;
			input.value = '';
		}
	}
</script>

<TopBar title="Files">
	{#if connectedNodes.length > 0}
		<select
			bind:value={selectedNodeId}
			class="rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--fg)] outline-none focus:border-[var(--accent)]"
		>
			{#each connectedNodes as node (node.id)}
				<option value={node.id}>{node.name}</option>
			{/each}
		</select>
		<input bind:this={fileInput} type="file" class="hidden" onchange={handleUpload} />
		<button
			onclick={() => fileInput?.click()}
			disabled={uploading}
			class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--accent-fg)] disabled:opacity-50"
		>
			{uploading ? 'Uploading…' : '+ Upload'}
		</button>
	{/if}
</TopBar>

<main class="flex-1 p-6">
	{#if error}
		<div class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-500">{error}</div>
	{/if}

	{#if connectedNodes.length === 0}
		<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
			<p class="text-[var(--fg-muted)]">No connected servers yet.</p>
		</div>
	{:else}
		<nav class="mb-3 flex flex-wrap items-center gap-1 text-sm text-[var(--fg-muted)]">
			{#each breadcrumbs as crumb, i (crumb.path)}
				{#if i > 0}<span class="text-[var(--fg-subtle)]">/</span>{/if}
				<button
					onclick={() => {
						currentPath = crumb.path;
						loadEntries();
					}}
					class="rounded-lg px-1.5 py-0.5 hover:bg-[var(--track)] hover:text-[var(--fg)]"
				>
					{crumb.label}
				</button>
			{/each}
		</nav>

		{#if loading}
			<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
				<p class="text-[var(--fg-muted)]">Loading…</p>
			</div>
		{:else if entries.length === 0}
			<div class="grid place-items-center rounded-2xl border border-dashed border-[var(--border)] py-24 text-center">
				<p class="text-[var(--fg-muted)]">This folder is empty.</p>
			</div>
		{:else}
			<div class="overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)]">
				<table class="w-full text-left text-sm">
					<thead class="border-b border-[var(--border)] bg-[var(--track)] text-xs uppercase tracking-wide text-[var(--fg-subtle)]">
						<tr>
							<th class="px-5 py-3 font-semibold" scope="col">Name</th>
							<th class="px-5 py-3 font-semibold" scope="col">Size</th>
							<th class="px-5 py-3 font-semibold" scope="col">Modified</th>
							<th class="px-5 py-3 font-semibold" scope="col">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border)]">
						{#each entries as entry (entry.name)}
							<tr class="text-[var(--fg-muted)] transition-colors hover:bg-[var(--track)]">
								<td class="px-5 py-3">
									{#if entry.isDir}
										<button onclick={() => open(entry)} class="flex items-center gap-2 font-semibold text-[var(--fg)] hover:text-[var(--accent)]">
											<svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2">
												<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z" />
											</svg>
											{entry.name}
										</button>
									{:else}
										<span class="flex items-center gap-2 text-[var(--fg)]">
											<svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2">
												<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6Z" />
												<path d="M14 2v6h6" />
											</svg>
											{entry.name}
										</span>
									{/if}
								</td>
								<td class="whitespace-nowrap px-5 py-3">{entry.isDir ? '—' : formatBytes(entry.size)}</td>
								<td class="whitespace-nowrap px-5 py-3">{new Date(entry.modTime).toLocaleString()}</td>
								<td class="whitespace-nowrap px-5 py-3">
									<div class="flex gap-2">
										{#if !entry.isDir && selectedNodeId}
											<a
												href={fileDownloadUrl(selectedNodeId, joinPath(currentPath, entry.name))}
												class="rounded-lg border border-[var(--border)] px-2.5 py-1 text-xs font-semibold hover:bg-[var(--surface-raised)]"
											>
												Download
											</a>
										{/if}
										<button
											onclick={() => remove(entry)}
											class="rounded-lg border border-[var(--border)] px-2.5 py-1 text-xs font-semibold text-rose-500 hover:bg-rose-500/10"
										>
											Delete
										</button>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</main>
