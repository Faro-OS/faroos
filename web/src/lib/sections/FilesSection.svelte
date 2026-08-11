<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';
	import folderIcon from '$lib/assets/dock/files.png';
	import { formatBytes } from '$lib/format';
	import {
		createDirectory,
		deleteFile,
		fileDownloadUrl,
		listFiles,
		listNodes,
		readFile,
		renameFile,
		uploadFile,
		type FileEntry,
		type Node
	} from '$lib/api';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	type ViewMode = 'list' | 'grid';
	const locations = [
		{ label: 'System', path: '/', icon: 'system' },
		{ label: 'Home', path: '/home', icon: 'home' },
		{ label: 'Root', path: '/root', icon: 'home' },
		{ label: 'Configuration', path: '/etc', icon: 'settings' },
		{ label: 'Services', path: '/var', icon: 'services' },
		{ label: 'Mounts', path: '/mnt', icon: 'disk' },
		{ label: 'Shared data', path: '/srv', icon: 'folder' },
		{ label: 'Applications', path: '/opt', icon: 'apps' }
	];
	const editableExtensions = new Set([
		'txt', 'md', 'json', 'yaml', 'yml', 'toml', 'ini', 'conf', 'config', 'env', 'xml', 'html', 'css',
		'js', 'ts', 'svelte', 'go', 'py', 'sh', 'service', 'log', 'csv', 'sql'
	]);

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let currentPath = $state('/');
	let addressPath = $state('/');
	let entries = $state<FileEntry[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let viewMode = $state<ViewMode>('list');
	let selectedEntry = $state<FileEntry | null>(null);
	let uploading = $state(false);
	let uploadProgress = $state('');
	let dragActive = $state(false);
	let fileInput = $state<HTMLInputElement>();
	let showNewFolder = $state(false);
	let newFolderName = $state('');
	let renameTarget = $state<FileEntry | null>(null);
	let renameValue = $state('');
	let editorTarget = $state<FileEntry | null>(null);
	let editorContent = $state('');
	let editorLoading = $state(false);
	let editorSaving = $state(false);

	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const filteredEntries = $derived(
		entries.filter((entry) => entry.name.toLowerCase().includes(search.trim().toLowerCase()))
	);
	const breadcrumbs = $derived.by(() => {
		const parts = currentPath.split('/').filter(Boolean);
		const result: { label: string; path: string }[] = [{ label: 'System', path: '/' }];
		let path = '';
		for (const part of parts) {
			path += `/${part}`;
			result.push({ label: part, path });
		}
		return result;
	});

	async function loadNodes() {
		try {
			nodes = await listNodes();
			if (!nodes.some((node) => node.id === selectedNodeId)) selectedNodeId = connectedNodes[0]?.id ?? null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not load servers';
		}
	}

	async function loadEntries(path = currentPath) {
		if (!selectedNodeId) {
			entries = [];
			loading = false;
			return;
		}
		loading = true;
		try {
			entries = await listFiles(selectedNodeId, path);
			currentPath = path;
			addressPath = path;
			selectedEntry = null;
			error = null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not open this location';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		(async () => {
			await loadNodes();
			await loadEntries('/');
		})();
	});

	$effect(() => {
		selectedNodeId;
		currentPath = '/';
		addressPath = '/';
		void loadEntries('/');
	});

	function cleanPath(value: string): string {
		const parts: string[] = [];
		for (const part of value.trim().split('/')) {
			if (!part || part === '.') continue;
			if (part === '..') parts.pop();
			else parts.push(part);
		}
		return `/${parts.join('/')}`;
	}

	function joinPath(base: string, name: string): string {
		return base === '/' ? `/${name}` : `${base}/${name}`;
	}

	function navigate(path: string) {
		void loadEntries(cleanPath(path));
	}

	function navigateAddress(event: SubmitEvent) {
		event.preventDefault();
		navigate(addressPath);
	}

	function goUp() {
		if (currentPath === '/') return;
		navigate(currentPath.slice(0, currentPath.lastIndexOf('/')) || '/');
	}

	function extension(name: string): string {
		return name.includes('.') ? name.split('.').pop()?.toLowerCase() ?? '' : '';
	}

	function canEdit(entry: FileEntry): boolean {
		return !entry.isDir && entry.size <= 2 * 1024 * 1024 && (editableExtensions.has(extension(entry.name)) || !entry.name.includes('.'));
	}

	async function openEntry(entry: FileEntry) {
		selectedEntry = entry;
		if (entry.isDir) {
			navigate(joinPath(currentPath, entry.name));
			return;
		}
		if (canEdit(entry)) await openEditor(entry);
	}

	async function openEditor(entry: FileEntry) {
		if (!selectedNodeId) return;
		editorTarget = entry;
		editorLoading = true;
		try {
			editorContent = await readFile(selectedNodeId, joinPath(currentPath, entry.name));
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Could not read the file';
			toastError(message);
			editorTarget = null;
		} finally {
			editorLoading = false;
		}
	}

	async function saveEditor() {
		if (!selectedNodeId || !editorTarget) return;
		editorSaving = true;
		try {
			await uploadFile(selectedNodeId, joinPath(currentPath, editorTarget.name), new Blob([editorContent], { type: 'text/plain;charset=utf-8' }));
			await loadEntries();
			toastSuccess(`${editorTarget.name} saved`);
			editorTarget = null;
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Could not save the file');
		} finally {
			editorSaving = false;
		}
	}

	async function uploadFiles(files: File[]) {
		if (!selectedNodeId || files.length === 0) return;
		uploading = true;
		try {
			for (let index = 0; index < files.length; index++) {
				const file = files[index];
				uploadProgress = `${index + 1} of ${files.length} · ${file.name}`;
				await uploadFile(selectedNodeId, joinPath(currentPath, file.name), file);
			}
			await loadEntries();
			toastSuccess(`${files.length} ${files.length === 1 ? 'file' : 'files'} uploaded`);
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Upload failed');
		} finally {
			uploading = false;
			uploadProgress = '';
			if (fileInput) fileInput.value = '';
		}
	}

	function handleUpload(event: Event) {
		const input = event.target as HTMLInputElement;
		void uploadFiles(Array.from(input.files ?? []));
	}

	function handleDrop(event: DragEvent) {
		event.preventDefault();
		dragActive = false;
		void uploadFiles(Array.from(event.dataTransfer?.files ?? []));
	}

	async function createFolder(event: SubmitEvent) {
		event.preventDefault();
		if (!selectedNodeId || !newFolderName.trim()) return;
		try {
			await createDirectory(selectedNodeId, joinPath(currentPath, newFolderName.trim()));
			await loadEntries();
			toastSuccess(`${newFolderName.trim()} created`);
			showNewFolder = false;
			newFolderName = '';
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Could not create the folder');
		}
	}

	function startRename(entry: FileEntry) {
		renameTarget = entry;
		renameValue = entry.name;
	}

	async function submitRename(event: SubmitEvent) {
		event.preventDefault();
		if (!selectedNodeId || !renameTarget || !renameValue.trim() || renameValue.includes('/')) return;
		try {
			await renameFile(selectedNodeId, joinPath(currentPath, renameTarget.name), joinPath(currentPath, renameValue.trim()));
			await loadEntries();
			toastSuccess('Item renamed');
			renameTarget = null;
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Could not rename the item');
		}
	}

	async function remove(entry: FileEntry) {
		if (!selectedNodeId || !confirm(`Delete “${entry.name}”? This cannot be undone.`)) return;
		try {
			await deleteFile(selectedNodeId, joinPath(currentPath, entry.name));
			await loadEntries();
			toastSuccess(`${entry.name} deleted`);
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Could not delete the item');
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			showNewFolder = false;
			renameTarget = null;
			editorTarget = null;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<TopBar title="Files">
	{#if connectedNodes.length > 0}
		<select bind:value={selectedNodeId} class="control node-select"><option value={null} disabled>Select server</option>{#each connectedNodes as node (node.id)}<option value={node.id}>{node.name}</option>{/each}</select>
		<input bind:this={fileInput} type="file" multiple class="hidden" onchange={handleUpload} />
		<button type="button" class="control toolbar-button" onclick={() => (showNewFolder = true)}><span>＋</span>Folder</button>
		<button type="button" class="primary-control toolbar-button" onclick={() => fileInput?.click()} disabled={uploading}>{uploading ? uploadProgress || 'Uploading…' : 'Upload'}</button>
	{/if}
</TopBar>

<main class="files-page section-enter">
	{#if error}<div class="files-error"><span>!</span><p>{error}</p><button type="button" onclick={() => loadEntries()}>Try again</button></div>{/if}
	{#if connectedNodes.length === 0}
		<section class="no-server"><div class="empty-icon">⌁</div><h2>No connected servers</h2><p>Add or reconnect a server to browse its complete filesystem.</p></section>
	{:else}
		<section class="finder-window" aria-label="Server filesystem" ondragover={(event) => { event.preventDefault(); dragActive = true; }} ondragleave={() => (dragActive = false)} ondrop={handleDrop}>
			<aside class="finder-sidebar">
				<p>Locations</p>
				<nav>{#each locations as location (location.path)}<button type="button" class:active={currentPath === location.path || (location.path !== '/' && currentPath.startsWith(`${location.path}/`))} onclick={() => navigate(location.path)}>
					<span class="location-icon {location.icon}">{#if location.icon === 'system'}<svg viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="4"/><path d="M8 9h8M8 13h8M8 17h5"/></svg>{:else if location.icon === 'home'}<svg viewBox="0 0 24 24"><path d="m4 11 8-7 8 7v9h-6v-6h-4v6H4v-9Z"/></svg>{:else if location.icon === 'settings'}<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="m19 12 2-1-2-4-2 .6a8 8 0 0 0-2-1.2L14 4h-4L9 6.4a8 8 0 0 0-2 1.2L5 7l-2 4 2 1a8 8 0 0 0 0 2l-2 1 2 4 2-.6a8 8 0 0 0 2 1.2L10 22h4l1-2.4a8 8 0 0 0 2-1.2l2 .6 2-4-2-1a8 8 0 0 0 0-2Z"/></svg>{:else if location.icon === 'apps'}<svg viewBox="0 0 24 24"><rect x="4" y="4" width="6" height="6" rx="2"/><rect x="14" y="4" width="6" height="6" rx="2"/><rect x="4" y="14" width="6" height="6" rx="2"/><rect x="14" y="14" width="6" height="6" rx="2"/></svg>{:else}<img src={folderIcon} alt="" />{/if}</span>
					<span>{location.label}</span>
				</button>{/each}</nav>
				<div class="access-note"><span>Full access</span><p>Browsing the complete filesystem of this server.</p></div>
			</aside>

			<div class="finder-main">
				<header class="finder-toolbar">
					<div class="nav-buttons"><button type="button" aria-label="Parent folder" onclick={goUp} disabled={currentPath === '/'}>‹</button><button type="button" aria-label="Refresh" onclick={() => loadEntries()}><svg viewBox="0 0 24 24"><path d="M20 7v5h-5M4 17v-5h5M6.5 9a6.5 6.5 0 0 1 11-2L20 9M4 15l2.5 2a6.5 6.5 0 0 0 11-2"/></svg></button></div>
					<form class="path-bar" onsubmit={navigateAddress}><img class="path-folder-icon" src={folderIcon} alt="" /><input bind:value={addressPath} aria-label="Filesystem path" spellcheck="false" /></form>
					<div class="view-switch"><button type="button" class:active={viewMode === 'list'} aria-label="List view" onclick={() => (viewMode = 'list')}><svg viewBox="0 0 24 24"><path d="M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01"/></svg></button><button type="button" class:active={viewMode === 'grid'} aria-label="Grid view" onclick={() => (viewMode = 'grid')}><svg viewBox="0 0 24 24"><rect x="4" y="4" width="6" height="6"/><rect x="14" y="4" width="6" height="6"/><rect x="4" y="14" width="6" height="6"/><rect x="14" y="14" width="6" height="6"/></svg></button></div>
					<label class="file-search"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="6"/><path d="m16 16 4 4"/></svg><input bind:value={search} placeholder="Search here" /></label>
				</header>

				<div class="breadcrumb-row"><nav aria-label="Current path">{#each breadcrumbs as crumb, index (crumb.path)}{#if index > 0}<span>›</span>{/if}<button type="button" onclick={() => navigate(crumb.path)}>{crumb.label}</button>{/each}</nav><span>{filteredEntries.length} items</span></div>

				<div class="file-content" class:drop-active={dragActive}>
					{#if dragActive}<div class="drop-overlay"><span>⇧</span><strong>Drop files to upload</strong><small>{currentPath}</small></div>{/if}
					{#if loading}<div class="loading-files"><span></span><p>Opening {currentPath}</p></div>
					{:else if filteredEntries.length === 0}<div class="empty-folder"><span><img src={folderIcon} alt="" /></span><h3>{search ? 'No matching files' : 'This folder is empty'}</h3><p>{search ? 'Try another search.' : 'Drop files here or create a new folder.'}</p></div>
					{:else if viewMode === 'list'}
						<div class="file-table"><div class="file-table-head"><span>Name</span><span>Size</span><span>Modified</span><span>Permissions</span><span></span></div>
							{#each filteredEntries as entry (entry.name)}
								<div class:selected={selectedEntry?.name === entry.name} class="file-row" ondblclick={() => openEntry(entry)} onclick={() => (selectedEntry = entry)} onkeydown={(event) => { if (event.key === 'Enter') void openEntry(entry); }} role="button" tabindex="0">
									<div class="name-cell"><span class:folder={entry.isDir} class="item-icon">{#if entry.isDir}<img src={folderIcon} alt="" />{:else}<svg viewBox="0 0 28 32"><path d="M5 1h12l7 7v23H5V1Z"/><path d="M17 1v8h7"/></svg>{/if}</span><span><strong>{entry.name}</strong>{#if entry.isSymlink}<small>Symbolic link</small>{/if}</span></div>
									<span>{entry.isDir ? '—' : formatBytes(entry.size)}</span><span>{new Date(entry.modTime).toLocaleString()}</span><code>{entry.mode || '—'}</code>
									<div class="row-actions">{#if !entry.isDir}<a href={fileDownloadUrl(selectedNodeId!, joinPath(currentPath, entry.name))} title="Download" onclick={(event) => event.stopPropagation()}><svg viewBox="0 0 24 24"><path d="M12 3v12m-4-4 4 4 4-4M5 20h14"/></svg></a>{/if}{#if canEdit(entry)}<button type="button" title="Edit" onclick={(event) => { event.stopPropagation(); void openEditor(entry); }}><svg viewBox="0 0 24 24"><path d="m4 16-1 5 5-1L20 8l-4-4L4 16Zm9-9 4 4"/></svg></button>{/if}<button type="button" title="Rename" onclick={(event) => { event.stopPropagation(); startRename(entry); }}><svg viewBox="0 0 24 24"><path d="M5 5h14M12 5v14m-4 0h8"/></svg></button><button type="button" class="delete" title="Delete" onclick={(event) => { event.stopPropagation(); void remove(entry); }}><svg viewBox="0 0 24 24"><path d="M5 7h14m-9-3h4l1 3m2 0-1 14H8L7 7m4 4v6m3-6v6"/></svg></button></div>
								</div>
							{/each}
						</div>
					{:else}
						<div class="file-grid">{#each filteredEntries as entry (entry.name)}<button type="button" class:selected={selectedEntry?.name === entry.name} onclick={() => (selectedEntry = entry)} ondblclick={() => openEntry(entry)}><span class:folder={entry.isDir} class="grid-icon">{#if entry.isDir}<img src={folderIcon} alt="" />{:else}<svg viewBox="0 0 28 32"><path d="M5 1h12l7 7v23H5V1Z"/><path d="M17 1v8h7"/></svg>{/if}</span><strong>{entry.name}</strong><small>{entry.isDir ? 'Folder' : formatBytes(entry.size)}</small></button>{/each}</div>
					{/if}
				</div>

				<footer class="finder-status"><span>{currentPath}</span>{#if selectedEntry}<span><b>{selectedEntry.name}</b> · {selectedEntry.isDir ? 'Folder' : formatBytes(selectedEntry.size)} · {selectedEntry.mode || 'permissions unavailable'}</span>{:else}<span>{entries.filter((entry) => entry.isDir).length} folders · {entries.filter((entry) => !entry.isDir).length} files</span>{/if}</footer>
			</div>
		</section>
	{/if}
</main>

{#if showNewFolder || renameTarget}
	<div class="file-modal-layer"><button type="button" class="modal-backdrop" aria-label="Close" onclick={() => { showNewFolder = false; renameTarget = null; }}></button><div class="small-file-modal"><span class="modal-folder"><img src={folderIcon} alt="" /></span><h2>{renameTarget ? 'Rename item' : 'New folder'}</h2><p>{renameTarget ? `Choose a new name for “${renameTarget.name}”.` : `Create inside ${currentPath}.`}</p><form onsubmit={renameTarget ? submitRename : createFolder}>{#if renameTarget}<input bind:value={renameValue} placeholder={renameTarget.name} />{:else}<input bind:value={newFolderName} placeholder="Untitled folder" />{/if}<div><button type="button" onclick={() => { showNewFolder = false; renameTarget = null; }}>Cancel</button><button type="submit" class="modal-primary">{renameTarget ? 'Rename' : 'Create'}</button></div></form></div></div>
{/if}

{#if editorTarget}
	<div class="file-modal-layer editor-layer"><button type="button" class="modal-backdrop" aria-label="Close editor" onclick={() => (editorTarget = null)}></button><section class="editor-modal"><header><div><span class="document-symbol">{extension(editorTarget.name).slice(0, 3).toUpperCase() || 'TXT'}</span><span><strong>{editorTarget.name}</strong><small>{joinPath(currentPath, editorTarget.name)}</small></span></div><button type="button" onclick={() => (editorTarget = null)}>×</button></header>{#if editorLoading}<div class="editor-loading">Loading file…</div>{:else}<textarea bind:value={editorContent} spellcheck="false" aria-label={`Edit ${editorTarget.name}`}></textarea>{/if}<footer><span>{new Blob([editorContent]).size.toLocaleString()} bytes</span><div><a href={fileDownloadUrl(selectedNodeId!, joinPath(currentPath, editorTarget.name))}>Download</a><button type="button" class="modal-primary" onclick={saveEditor} disabled={editorSaving}>{editorSaving ? 'Saving…' : 'Save changes'}</button></div></footer></section></div>
{/if}

<style>
	.node-select { height: 39px; padding: 0 12px; border-radius: 12px; color: var(--fg); font-size: 12px; outline: none; }.toolbar-button { height: 39px; min-height: 39px; padding: 0 14px; border: 0; border-radius: 12px; font-size: 11px; font-weight: 650; }.toolbar-button span { margin-right: 4px; font-size: 16px; }.toolbar-button.control { border: 1px solid var(--border); }
	.files-page { width: 100%; max-width: 1480px; margin: 0 auto; padding: 24px 40px 126px; }.files-error { display: flex; align-items: center; gap: 11px; margin-bottom: 14px; padding: 11px 14px; border: 1px solid rgba(220,67,77,.12); border-radius: 14px; color: #c63f49; background: rgba(220,67,77,.055); font-size: 12px; }.files-error > span { width: 20px; height: 20px; display: grid; place-items: center; border-radius: 50%; color: #fff; background: #d84e58; font-weight: 700; }.files-error p { flex:1; margin:0; }.files-error button { border:0; background:transparent; color:inherit; font-weight:650; }.no-server { min-height: 520px; display: grid; place-content: center; text-align: center; }.empty-icon { margin:auto; font-size:42px; color:var(--fg-subtle); }.no-server h2 { margin:13px 0 5px; }.no-server p { margin:0; color:var(--fg-subtle); font-size:13px; }
	.finder-window { width:100%; height:min(720px,calc(100dvh - 228px)); min-height:420px; display:grid; grid-template-columns:205px minmax(0,1fr); overflow:hidden; border:1px solid rgba(22,28,38,.08); border-radius:24px; color:#272a30; background:rgba(255,255,255,.89); box-shadow:0 24px 70px rgba(15,23,42,.09),0 2px 8px rgba(15,23,42,.035); backdrop-filter:blur(28px); }
	.finder-sidebar { min-height:0; display:flex; flex-direction:column; overflow-y:auto; padding:20px 12px 15px; border-right:1px solid rgba(26,32,43,.075); background:linear-gradient(165deg,rgba(243,245,247,.9),rgba(237,240,243,.76)); }.finder-sidebar > p { margin:0 0 8px 12px; color:#979ca4; font-size:10px; font-weight:700; letter-spacing:.09em; text-transform:uppercase; }.finder-sidebar nav { display:flex; flex-direction:column; gap:2px; }.finder-sidebar nav button { height:36px; display:flex; flex:0 0 auto; align-items:center; gap:10px; padding:0 11px; border:0; border-radius:9px; color:#50555d; background:transparent; font-size:11px; text-align:left; }.finder-sidebar nav button:hover { background:rgba(255,255,255,.62); }.finder-sidebar nav button.active { color:#1f5da9; background:rgba(52,120,246,.12); font-weight:620; }.location-icon { width:19px; height:19px; display:grid; place-items:center; color:#737982; }.location-icon svg { width:18px; height:18px; fill:none; stroke:currentColor; stroke-width:1.55; stroke-linecap:round; stroke-linejoin:round; }.location-icon img { width:19px; height:19px; border-radius:5px; object-fit:cover; }.finder-sidebar button.active .location-icon { color:#3478f6; }.access-note { margin-top:auto; padding:12px; border:1px solid rgba(52,120,246,.08); border-radius:12px; background:rgba(255,255,255,.54); }.access-note span { display:flex; align-items:center; gap:6px; color:#268b45; font-size:10px; font-weight:680; }.access-note span::before { width:6px; height:6px; content:''; border-radius:50%; background:#30b84a; }.access-note p { margin:5px 0 0; color:#969ba3; font-size:9px; line-height:1.45; }
	.finder-main { min-width:0; min-height:0; display:grid; grid-template-rows:60px 39px minmax(0,1fr) 34px; overflow:hidden; }.finder-toolbar { display:flex; align-items:center; gap:10px; padding:10px 13px; border-bottom:1px solid #eceef0; }.nav-buttons,.view-switch { display:flex; align-items:center; border:1px solid #e3e6e9; border-radius:9px; background:#f8f9fa; }.nav-buttons button,.view-switch button { width:34px; height:32px; display:grid; place-items:center; border:0; background:transparent; color:#686e77; }.nav-buttons button+button,.view-switch button+button { border-left:1px solid #e3e6e9; }.nav-buttons button:disabled { opacity:.35; }.nav-buttons svg,.view-switch svg { width:16px; height:16px; fill:none; stroke:currentColor; stroke-width:1.7; stroke-linecap:round; stroke-linejoin:round; }.view-switch button.active { color:#2f3339; background:#fff; box-shadow:0 2px 6px rgba(15,23,42,.06); }.path-bar { min-width:170px; flex:1; height:34px; display:flex; align-items:center; gap:8px; padding:0 10px; border:1px solid #e2e5e8; border-radius:9px; background:#fafbfc; }.path-bar .path-folder-icon { width:18px; height:18px; flex:0 0 auto; border-radius:5px; object-fit:cover; }.file-search svg { width:15px; height:15px; flex:0 0 auto; fill:none; stroke:#858b94; stroke-width:1.6; stroke-linecap:round; stroke-linejoin:round; }.path-bar input,.file-search input { min-width:0; flex:1; border:0; color:#3f444b; background:transparent; font:11px ui-monospace,SFMono-Regular,monospace; outline:none; }.file-search { width:180px; height:34px; display:flex; align-items:center; gap:7px; padding:0 9px; border:1px solid #e2e5e8; border-radius:9px; background:#fafbfc; }.file-search input { font-family:inherit; }.breadcrumb-row { display:flex; align-items:center; justify-content:space-between; gap:15px; padding:0 16px; border-bottom:1px solid #eff1f2; color:#989da5; font-size:9px; }.breadcrumb-row nav { min-width:0; display:flex; align-items:center; overflow:hidden; }.breadcrumb-row nav button { max-width:150px; overflow:hidden; padding:4px 5px; border:0; border-radius:5px; color:#5f646d; background:transparent; font-size:10px; text-overflow:ellipsis; white-space:nowrap; }.breadcrumb-row nav button:hover { background:#f1f3f5; }.breadcrumb-row nav span { color:#a9adb3; }
	.file-content { position:relative; width:100%; height:100%; min-height:0; overflow:auto; overscroll-behavior:contain; scrollbar-gutter:stable; -webkit-overflow-scrolling:touch; background:rgba(255,255,255,.56); }.drop-overlay { position:absolute; z-index:10; inset:14px; display:grid; place-content:center; border:2px dashed rgba(52,120,246,.4); border-radius:16px; color:#286dc7; background:rgba(240,247,255,.92); text-align:center; backdrop-filter:blur(8px); }.drop-overlay span { font-size:36px; }.drop-overlay strong { margin-top:7px; font-size:14px; }.drop-overlay small { margin-top:4px; color:#7a9bc3; }.loading-files,.empty-folder { height:100%; display:grid; place-content:center; justify-items:center; color:#979ca4; text-align:center; }.loading-files span { width:24px; height:24px; border:2px solid #e1e4e8; border-top-color:#3478f6; border-radius:50%; animation:spin .8s linear infinite; }.loading-files p { font-size:11px; }.empty-folder > span { width:64px; height:56px; display:grid; place-items:center; }.empty-folder img { width:56px; height:56px; border-radius:14px; object-fit:cover; }.empty-folder h3 { margin:7px 0 3px; color:#535861; font-size:13px; }.empty-folder p { margin:0; font-size:10px; }
	.file-table { min-width:820px; }.file-table-head,.file-row { display:grid; grid-template-columns:minmax(280px,1.6fr) 90px 165px 118px 142px; align-items:center; }.file-table-head { position:sticky; z-index:2; top:0; height:31px; padding:0 12px; border-bottom:1px solid #e9ebed; color:#969ba3; background:rgba(248,249,250,.94); font-size:9px; font-weight:650; backdrop-filter:blur(14px); }.file-row { min-height:48px; padding:0 12px; border-bottom:1px solid #f0f1f2; color:#737881; font-size:10px; transition:background .12s ease; }.file-row:hover,.file-row.selected { background:#f3f7fc; }.file-row.selected { box-shadow:inset 3px 0 #3478f6; }.name-cell { min-width:0; display:flex; align-items:center; gap:10px; }.name-cell > span:last-child { min-width:0; display:flex; flex-direction:column; }.name-cell strong { overflow:hidden; color:#363a41; font-size:11px; font-weight:590; text-overflow:ellipsis; white-space:nowrap; }.name-cell small { margin-top:2px; color:#9da2a9; font-size:8px; }.item-icon { width:29px; height:29px; display:grid; flex:0 0 auto; place-items:center; }.item-icon svg { width:24px; height:27px; fill:#f4f5f6; stroke:#9298a1; stroke-width:1.1; stroke-linejoin:round; }.item-icon.folder img { width:29px; height:29px; border-radius:7px; object-fit:cover; }.file-row code { color:#80858d; font:9px ui-monospace,SFMono-Regular,monospace; }.row-actions { display:flex; align-items:center; justify-content:flex-end; gap:2px; opacity:0; }.file-row:hover .row-actions,.file-row.selected .row-actions { opacity:1; }.row-actions button,.row-actions a { width:28px; height:28px; display:grid; place-items:center; border:0; border-radius:7px; color:#747a83; background:transparent; }.row-actions button:hover,.row-actions a:hover { color:#3478f6; background:#e7f0fc; }.row-actions .delete:hover { color:#d64550; background:#fcebed; }.row-actions svg { width:14px; height:14px; fill:none; stroke:currentColor; stroke-width:1.6; stroke-linecap:round; stroke-linejoin:round; }
	.file-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(120px,1fr)); gap:8px; padding:17px; }.file-grid button { min-width:0; height:118px; display:flex; align-items:center; flex-direction:column; justify-content:center; padding:9px; border:1px solid transparent; border-radius:12px; background:transparent; }.file-grid button:hover { background:#f7f9fb; }.file-grid button.selected { border-color:rgba(52,120,246,.2); background:#edf5ff; }.grid-icon { width:53px; height:50px; display:grid; place-items:center; }.grid-icon svg { width:40px; height:45px; fill:#f4f5f6; stroke:#9399a1; stroke-width:1.1; stroke-linejoin:round; }.grid-icon.folder img { width:52px; height:52px; border-radius:13px; object-fit:cover; }.file-grid strong { width:100%; overflow:hidden; margin-top:7px; color:#41454c; font-size:10px; font-weight:590; text-overflow:ellipsis; white-space:nowrap; }.file-grid small { margin-top:3px; color:#a0a5ad; font-size:8px; }
	.finder-status { display:flex; align-items:center; justify-content:space-between; gap:15px; padding:0 15px; border-top:1px solid #eceef0; color:#979ca4; background:#fafbfc; font-size:9px; }.finder-status > span { max-width:50%; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.finder-status b { color:#626770; font-weight:590; }
	.file-modal-layer { position:fixed; z-index:80; inset:0; display:grid; place-items:center; padding:18px; }.modal-backdrop { position:absolute; inset:0; border:0; background:rgba(24,28,34,.3); backdrop-filter:blur(10px); }.small-file-modal { position:relative; width:min(390px,100%); padding:25px; border:1px solid rgba(0,0,0,.07); border-radius:22px; color:#292c31; background:rgba(255,255,255,.97); box-shadow:0 30px 80px rgba(15,23,42,.17); }.modal-folder { width:48px;height:48px;display:grid;place-items:center;}.modal-folder img { width:48px;height:48px;border-radius:12px;object-fit:cover; }.small-file-modal h2 { margin:12px 0 4px;font-size:19px;letter-spacing:-.025em; }.small-file-modal p { margin:0 0 18px;color:#8c9199;font-size:11px; }.small-file-modal form { display:flex;flex-direction:column;gap:13px; }.small-file-modal input { height:42px;padding:0 12px;border:1px solid #dde0e4;border-radius:11px;color:#31353b;background:#fafbfc;outline:none; }.small-file-modal form div { display:flex;justify-content:flex-end;gap:7px; }.small-file-modal button,.editor-modal footer button,.editor-modal footer a { padding:9px 13px;border:0;border-radius:9px;color:#686d75;background:transparent;font-size:10px;font-weight:640;text-decoration:none; }.modal-primary { color:#fff !important;background:#3478f6 !important; }
	.editor-modal { position:relative; width:min(980px,calc(100vw - 36px)); height:min(720px,calc(100dvh - 60px)); display:grid;grid-template-rows:61px minmax(0,1fr) 52px;overflow:hidden;border:1px solid rgba(0,0,0,.08);border-radius:22px;color:#292c31;background:#fff;box-shadow:0 35px 100px rgba(15,23,42,.2); }.editor-modal header { display:flex;align-items:center;justify-content:space-between;padding:0 18px;border-bottom:1px solid #e9ebed;background:#fafbfc; }.editor-modal header>div { min-width:0;display:flex;align-items:center;gap:11px; }.document-symbol { width:34px;height:38px;display:grid;place-items:center;border:1px solid #dce0e4;border-radius:8px;color:#3478f6;background:#fff;font-size:8px;font-weight:750; }.editor-modal header span:last-child { min-width:0;display:flex;flex-direction:column; }.editor-modal header strong { color:#373b42;font-size:12px; }.editor-modal header small { max-width:650px;overflow:hidden;margin-top:3px;color:#979ca4;font-size:9px;text-overflow:ellipsis;white-space:nowrap; }.editor-modal header>button { width:32px;height:32px;border:0;border-radius:9px;color:#777c84;background:transparent;font-size:22px; }.editor-modal header>button:hover { background:#eef0f2; }.editor-modal textarea { width:100%;height:100%;resize:none;padding:22px 24px;border:0;color:#25282d;background:#fff;font:12px/1.65 ui-monospace,SFMono-Regular,Menlo,Monaco,monospace;outline:none;tab-size:4; }.editor-loading { display:grid;place-content:center;color:#9297a0;font-size:12px; }.editor-modal footer { display:flex;align-items:center;justify-content:space-between;padding:0 16px;border-top:1px solid #e9ebed;background:#fafbfc;color:#999ea6;font-size:9px; }.editor-modal footer>div { display:flex;align-items:center;gap:5px; }.editor-modal footer a { border:1px solid #e0e3e7;background:#fff; }

	/* Finder materials inherit the platform appearance instead of forcing light mode. */
	.finder-window { border-color: var(--border); color: var(--fg); background: color-mix(in srgb,var(--surface-raised) 90%,transparent); box-shadow: inset 0 1px 0 color-mix(in srgb,white 18%,transparent),var(--shadow-md); backdrop-filter: blur(30px) saturate(1.45); -webkit-backdrop-filter: blur(30px) saturate(1.45); }
	.finder-sidebar { border-color: var(--border); background: linear-gradient(165deg,color-mix(in srgb,var(--sidebar-bg) 94%,transparent),color-mix(in srgb,var(--bg-elevated) 78%,transparent)); }
	.finder-sidebar > p,.access-note p,.breadcrumb-row,.loading-files,.empty-folder,.file-table-head,.file-row,.file-row code,.file-grid small,.finder-status,.editor-loading { color: var(--fg-subtle); }
	.finder-sidebar nav button { color: var(--fg-muted); }.finder-sidebar nav button:hover { background: var(--surface); }.finder-sidebar nav button.active { color: var(--accent); background: var(--accent-soft); }.finder-sidebar button.active .location-icon { color: var(--accent); }
	.access-note { border-color: color-mix(in srgb,var(--accent) 13%,var(--border)); background: var(--surface); }.access-note span { color: var(--success); }.access-note span::before { background: var(--success); }
	.finder-toolbar,.breadcrumb-row { border-color: var(--border); }.nav-buttons,.view-switch,.path-bar,.file-search { border-color: var(--border); background: var(--track); }.nav-buttons button+button,.view-switch button+button { border-color: var(--border); }.nav-buttons button,.view-switch button { color: var(--fg-muted); }.view-switch button.active { color: var(--fg); background: var(--surface-raised); box-shadow: var(--shadow-sm); }
	.path-bar input,.file-search input { color: var(--fg); }.file-search svg { stroke: var(--fg-subtle); }.breadcrumb-row nav button { color: var(--fg-muted); }.breadcrumb-row nav button:hover { background: var(--track); }.breadcrumb-row nav span { color: var(--fg-subtle); }
	.file-content { background: color-mix(in srgb,var(--surface) 72%,transparent); }.drop-overlay { border-color: color-mix(in srgb,var(--accent) 42%,transparent); color: var(--accent); background: color-mix(in srgb,var(--accent-soft) 76%,var(--surface-raised)); }.drop-overlay small { color: var(--fg-muted); }
	.loading-files span { border-color: var(--border); border-top-color: var(--accent); }.empty-folder h3 { color: var(--fg-muted); }
	.file-table-head { border-color: var(--border); background: color-mix(in srgb,var(--chrome) 95%,transparent); }.file-row { border-color: var(--border); }.file-row:hover,.file-row.selected { background: var(--accent-soft); }.file-row.selected { box-shadow: inset 3px 0 var(--accent); }.name-cell strong,.file-grid strong { color: var(--fg); }.name-cell small { color: var(--fg-subtle); }.item-icon svg,.grid-icon svg { fill: var(--bg-elevated); stroke: var(--fg-subtle); }
	.row-actions button,.row-actions a { color: var(--fg-subtle); }.row-actions button:hover,.row-actions a:hover { color: var(--accent); background: var(--accent-soft); }.row-actions .delete:hover { color: var(--danger); background: color-mix(in srgb,var(--danger) 10%,transparent); }
	.file-grid button:hover { background: var(--track); }.file-grid button.selected { border-color: color-mix(in srgb,var(--accent) 28%,var(--border)); background: var(--accent-soft); }
	.finder-status { border-color: var(--border); background: color-mix(in srgb,var(--chrome) 94%,transparent); }.finder-status b { color: var(--fg-muted); }
	.modal-backdrop { background: color-mix(in srgb,#07080b 38%,transparent); animation: file-scrim 180ms ease-out both; backdrop-filter: blur(9px) saturate(.85); -webkit-backdrop-filter: blur(9px) saturate(.85); }
	.small-file-modal,.editor-modal { border-color: var(--border); color: var(--fg); background: color-mix(in srgb,var(--surface-raised) 98%,transparent); box-shadow: var(--shadow-xl); transform-origin: center 25%; animation: file-modal-in 340ms var(--motion-settle) both; backdrop-filter: blur(34px) saturate(1.4); -webkit-backdrop-filter: blur(34px) saturate(1.4); }
	.small-file-modal p { color: var(--fg-subtle); }.small-file-modal input { border-color: var(--border); color: var(--fg); background: var(--track); }.small-file-modal input:focus { border-color: color-mix(in srgb,var(--accent) 58%,var(--border)); box-shadow: 0 0 0 4px var(--accent-soft); }
	.small-file-modal button,.editor-modal footer button,.editor-modal footer a { color: var(--fg-muted); }
	.editor-modal header,.editor-modal footer { border-color: var(--border); background: color-mix(in srgb,var(--chrome) 94%,transparent); }.document-symbol { border-color: var(--border); color: var(--accent); background: var(--surface-solid); }.editor-modal header strong { color: var(--fg); }.editor-modal header small,.editor-modal header>button,.editor-modal footer { color: var(--fg-subtle); }.editor-modal header>button:hover { background: var(--track); }.editor-modal textarea { color: var(--fg); background: var(--surface-solid); }.editor-modal footer a { border-color: var(--border); background: var(--surface-raised); }

	@keyframes file-modal-in { from { opacity:0; transform:translateY(10px) scale(.97); filter:blur(7px); } to { opacity:1; transform:translateY(0) scale(1); filter:blur(0); } }
	@keyframes file-scrim { from { opacity:0; } to { opacity:1; } }
	@media (prefers-reduced-motion: reduce) { .small-file-modal,.editor-modal,.modal-backdrop { animation:none; } }
	@media (prefers-reduced-transparency: reduce) { .finder-window,.small-file-modal,.editor-modal { background:var(--surface-solid); backdrop-filter:none; -webkit-backdrop-filter:none; }.modal-backdrop { backdrop-filter:none; -webkit-backdrop-filter:none; } }
	@media (prefers-contrast: more) { .finder-window,.small-file-modal,.editor-modal { border-color:var(--border-strong); background:var(--surface-solid); }.finder-sidebar { background:var(--bg-elevated); } }
	@keyframes spin { to { transform:rotate(360deg); } }
	@media(max-width:900px){.files-page{padding:16px 16px 105px}.finder-window{height:calc(100dvh - 199px);min-height:360px;grid-template-columns:1fr}.finder-sidebar{display:none}.file-search{display:none}.file-table-head,.file-row{grid-template-columns:minmax(240px,1.5fr) 80px 145px 120px}.file-table-head span:nth-child(4),.file-row code{display:none}.finder-status span:first-child{display:none}.finder-status>span{max-width:100%}}
	@media(max-width:600px){.view-switch{display:none}.path-bar{min-width:100px}.finder-main{grid-template-rows:55px 36px minmax(0,1fr) 30px}.file-grid{grid-template-columns:repeat(3,minmax(0,1fr));padding:10px}.toolbar-button.control{display:none}.node-select{max-width:130px}.editor-modal{height:calc(100dvh - 26px)}.editor-modal textarea{padding:15px;font-size:11px}}
</style>
