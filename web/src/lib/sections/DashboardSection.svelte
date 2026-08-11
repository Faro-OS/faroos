<script lang="ts">
	import { onMount } from 'svelte';
	import AppIcon from '$lib/components/AppIcon.svelte';
	import { formatMemory, formatStorage, formatUptime, totalStorageBytes } from '$lib/format';
	import {
		containerAction,
		createPairing,
		listApps,
		listContainers,
		listFiles,
		listNodes,
		relayStatus,
		runInternetSpeedTest,
		type CatalogApp,
		type Container,
		type ContainerAction,
		type FileEntry,
		type InternetSpeedResult,
		type Node,
		type PairingResult
	} from '$lib/api';
	import { containerAppIcon, containerAppName, containerWebPort } from '$lib/containerApps';
	import folderIcon from '$lib/assets/dock/files.png';
	import serverDeviceImage from '$lib/assets/server-device.png';
	import { setSection } from '$lib/section.svelte';
	import { toastError, toastSuccess } from '$lib/toast.svelte';

	type LaunchableApp = {
		id: string;
		name: string;
		icon?: string;
		container: Container;
		port?: number;
	};
	type DashboardShortcut = {
		id: string;
		name: string;
		url: string;
		icon?: string;
	};
	type IconTarget = {
		kind: 'app' | 'shortcut';
		id: string;
		name: string;
		currentIcon: string;
	};
	type WidgetType = 'internet-speed' | 'html';
	type WidgetAccent = '#3478f6' | '#775cff' | '#20a56b' | '#f28b30';
	type DashboardWidget = {
		id: string;
		type: WidgetType;
		title: string;
		accent: WidgetAccent;
		html?: string;
	};
	type MetricPoint = {
		timestamp: string;
		cpu: number;
		memory: number;
		storage: number;
		receive: number;
		transmit: number;
	};
	type MetricKey = Exclude<keyof MetricPoint, 'timestamp'>;

	const widgetStorageKey = 'faroos:dashboard-widgets:v1';
	const shortcutStorageKey = 'faroos:dashboard-shortcuts:v1';
	const appIconStorageKey = 'faroos:dashboard-app-icons:v1';
	const appOrderStorageKey = 'faroos:dashboard-app-order:v1';
	const metricHistorySize = 60;
	const speedTestIntervalMs = 30_000;
	const widgetAccents: WidgetAccent[] = ['#3478f6', '#775cff', '#20a56b', '#f28b30'];
	const defaultWidgets: DashboardWidget[] = [
		{ id: 'internet-speed-default', type: 'internet-speed', title: 'Live Internet', accent: '#3478f6' }
	];
	const defaultHTMLWidget = `<style>
  body { margin: 0; min-height: 100vh; display: grid; place-items: center; font: 15px system-ui; color: #f5f5f7; background: linear-gradient(145deg, #1677ff, #775cff); }
  .card { text-align: center; padding: 24px; }
  strong { display: block; font-size: 32px; letter-spacing: -.04em; }
  span { opacity: .75; }
</style>
<div class="card"><strong>Hello, FaroOS</strong><span>Edit this HTML to build your widget.</span></div>`;

	let nodes = $state<Node[]>([]);
	let selectedNodeId = $state<string | null>(null);
	let catalog = $state<CatalogApp[]>([]);
	let containers = $state<Container[]>([]);
	let recentEntries = $state<FileEntry[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let clock = $state(new Date());
	let profileOpen = $state(false);
	let showPairModal = $state(false);
	let newNodeName = $state('');
	let panelAddress = $state('');
	let pairing = $state(false);
	let pairingResult = $state<PairingResult | null>(null);
	let relayEnabled = $state(false);
	let p2pEnabled = $state(false);
	let widgets = $state<DashboardWidget[]>(defaultWidgets);
	let widgetsReady = $state(false);
	let showWidgetModal = $state(false);
	let editingWidgetId = $state<string | null>(null);
	let widgetDraftTitle = $state('Live Internet');
	let widgetDraftAccent = $state<WidgetAccent>('#3478f6');
	let widgetDraftType = $state<WidgetType>('internet-speed');
	let widgetDraftHTML = $state(defaultHTMLWidget);
	let metricHistory = $state<Record<string, MetricPoint[]>>({});
	let pageVisible = $state(true);
	let speedTestResult = $state<{ nodeId: string; value: InternetSpeedResult } | null>(null);
	let speedTestError = $state<{ nodeId: string; message: string } | null>(null);
	let speedTestRunningNodeId = $state<string | null>(null);
	let speedTestInFlightNodeId: string | null = null;
	let openAppMenuId = $state<string | null>(null);
	let appActionInFlight = $state<{ id: string; action: ContainerAction } | null>(null);
	let dashboardShortcuts = $state<DashboardShortcut[]>([]);
	let customAppIcons = $state<Record<string, string>>({});
	let appOrder = $state<string[]>([]);
	let draggedAppId = $state<string | null>(null);
	let appDropTargetId = $state<string | null>(null);
	let customizationsReady = $state(false);
	let showShortcutModal = $state(false);
	let editingShortcutId = $state<string | null>(null);
	let shortcutDraftName = $state('');
	let shortcutDraftUrl = $state('');
	let shortcutDraftIcon = $state('');
	let shortcutIconFileLoading = $state(false);
	let iconTarget = $state<IconTarget | null>(null);
	let iconDraft = $state('');
	let iconFileLoading = $state(false);

	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const fleetStorage = $derived(connectedNodes.reduce((sum, node) => sum + totalStorageBytes(node.stats), 0));
	const selectedNode = $derived(
		nodes.find((node) => node.id === selectedNodeId) ?? connectedNodes[0] ?? nodes[0] ?? null
	);
	const selectedNodeConnected = $derived(Boolean(selectedNode?.connected));
	const statsAreLive = $derived.by(() => {
		if (!selectedNode?.connected || !selectedNode.stats.timestamp) return false;
		const age = clock.getTime() - new Date(selectedNode.stats.timestamp).getTime();
		return Number.isFinite(age) && age >= -2000 && age < 5000;
	});
	const cpuPercent = $derived(statsAreLive ? clampPercent(selectedNode?.stats.cpuPercent ?? 0) : 0);
	const memoryPercent = $derived(
		statsAreLive && selectedNode?.stats.memTotalBytes
			? Math.max(0, Math.min(100, (selectedNode.stats.memUsedBytes / selectedNode.stats.memTotalBytes) * 100))
			: 0
	);
	const storagePercent = $derived(
		statsAreLive && selectedNode?.stats.diskTotalBytes
			? Math.max(0, Math.min(100, (selectedNode.stats.diskUsedBytes / selectedNode.stats.diskTotalBytes) * 100))
			: 0
	);
	const selectedHistory = $derived(statsAreLive && selectedNodeId ? (metricHistory[selectedNodeId] ?? []) : []);
	const cpuPath = $derived(sparklinePath(selectedHistory, 'cpu'));
	const memoryPath = $derived(sparklinePath(selectedHistory, 'memory'));
	const storagePath = $derived(sparklinePath(selectedHistory, 'storage'));
	const selectedSpeedTest = $derived(
		speedTestResult?.nodeId === selectedNodeId ? speedTestResult.value : null
	);
	const selectedSpeedTestError = $derived(
		speedTestError?.nodeId === selectedNodeId ? speedTestError.message : null
	);
	const speedTestRunning = $derived(speedTestRunningNodeId === selectedNodeId);
	const pairingConnected = $derived(
		Boolean(pairingResult && nodes.find((node) => node.id === pairingResult?.id)?.connected)
	);
	const runningContainers = $derived(containers.filter((container) => container.state === 'running').length);
	const stoppedContainers = $derived(containers.length - runningContainers);
	const greeting = $derived(
		clock.getHours() < 12 ? 'Good morning' : clock.getHours() < 18 ? 'Good afternoon' : 'Good evening'
	);

	const launchableApps = $derived.by(() => {
		const matched = new Set<string>();
		const result: LaunchableApp[] = [];
		for (const app of catalog) {
			const container = containers.find((candidate) => candidate.names.includes(`/faroos-app-${app.id}`));
			if (!container) continue;
			matched.add(container.id);
			result.push({
				id: app.id,
				name: app.name,
				icon: app.icon || containerAppIcon(container),
				container,
				port: containerWebPort(container) ?? app.ports?.[0]?.host
			});
		}
		for (const container of containers) {
			const port = containerWebPort(container);
			if (matched.has(container.id) || !port) continue;
			result.push({
				id: container.id,
				name: containerAppName(container),
				icon: containerAppIcon(container),
				container,
				port
			});
		}
		return result.sort((a, b) => {
			if (a.container.state === b.container.state) return a.name.localeCompare(b.name);
			return a.container.state === 'running' ? -1 : 1;
		});
	});
	const orderedLaunchableApps = $derived.by(() => {
		const order = new Map(appOrder.map((id, index) => [id, index]));
		return [...launchableApps].sort((a, b) => {
			const aIndex = order.get(a.id);
			const bIndex = order.get(b.id);
			if (aIndex !== undefined && bIndex !== undefined) return aIndex - bIndex;
			if (aIndex !== undefined) return -1;
			if (bIndex !== undefined) return 1;
			return 0;
		});
	});
	const visibleDashboardShortcuts = $derived(dashboardShortcuts.slice(0, 8));
	const visibleDashboardApps = $derived(orderedLaunchableApps.slice(0, Math.max(0, 8 - visibleDashboardShortcuts.length)));
	const emptyDashboardAppSlots = $derived(
		Math.min(1, Math.max(0, 8 - visibleDashboardShortcuts.length - visibleDashboardApps.length))
	);

	const visibleRecentEntries = $derived(
		[...recentEntries]
			.sort((a, b) => new Date(b.modTime).getTime() - new Date(a.modTime).getTime())
			.slice(0, 5)
	);

	onMount(() => {
		try {
			const stored = localStorage.getItem(widgetStorageKey);
			if (stored !== null) widgets = normalizeWidgets(JSON.parse(stored));
		} catch {
			widgets = defaultWidgets;
		}
		try {
			dashboardShortcuts = normalizeShortcuts(JSON.parse(localStorage.getItem(shortcutStorageKey) ?? '[]'));
		} catch {
			dashboardShortcuts = [];
		}
		try {
			customAppIcons = normalizeIconMap(JSON.parse(localStorage.getItem(appIconStorageKey) ?? '{}'));
		} catch {
			customAppIcons = {};
		}
		try {
			appOrder = normalizeAppOrder(JSON.parse(localStorage.getItem(appOrderStorageKey) ?? '[]'));
		} catch {
			appOrder = [];
		}
		widgetsReady = true;
		customizationsReady = true;
		pageVisible = document.visibilityState === 'visible';
		const handleVisibilityChange = () => {
			pageVisible = document.visibilityState === 'visible';
		};
		document.addEventListener('visibilitychange', handleVisibilityChange);
		return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
	});

	$effect(() => {
		if (widgetsReady && typeof localStorage !== 'undefined') {
			try {
				localStorage.setItem(widgetStorageKey, JSON.stringify(widgets));
			} catch {
				// Keep the dashboard usable when custom HTML reaches the browser quota.
			}
		}
	});

	$effect(() => {
		if (!customizationsReady || typeof localStorage === 'undefined') return;
		try {
			localStorage.setItem(shortcutStorageKey, JSON.stringify(dashboardShortcuts));
			localStorage.setItem(appIconStorageKey, JSON.stringify(customAppIcons));
			localStorage.setItem(appOrderStorageKey, JSON.stringify(appOrder));
		} catch {
			// Keep the current session usable if browser storage is full or blocked.
		}
	});

	async function refreshNodes() {
		try {
			const nextNodes = await listNodes();
			recordMetricSamples(nextNodes);
			nodes = nextNodes;
			const currentStillExists = nextNodes.some((node) => node.id === selectedNodeId);
			if (!currentStillExists) {
				selectedNodeId = nextNodes.find((node) => node.connected)?.id ?? nextNodes[0]?.id ?? null;
			}
			loadError = null;
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'FaroOS could not reach the server';
		} finally {
			loading = false;
		}
	}

	async function refreshNodeContent() {
		if (!selectedNodeId) {
			containers = [];
			recentEntries = [];
			return;
		}
		const [nextContainers, nextFiles] = await Promise.all([
			listContainers(selectedNodeId).catch(() => []),
			listFiles(selectedNodeId, '/').catch(() => [])
		]);
		containers = nextContainers;
		recentEntries = nextFiles;
	}

	$effect(() => {
		(async () => {
			const [nextCatalog, network] = await Promise.all([
				listApps().catch(() => []),
				relayStatus().catch(() => ({ enabled: false, publicUrl: '', p2p: false }))
			]);
			catalog = nextCatalog;
			relayEnabled = network.enabled;
			p2pEnabled = network.p2p;
			await refreshNodes();
			await refreshNodeContent();
		})();
		const statsTimer = setInterval(() => {
			void refreshNodes();
		}, 1000);
		const contentTimer = setInterval(() => {
			void refreshNodeContent();
		}, 5000);
		const clockTimer = setInterval(() => (clock = new Date()), 1000);
		return () => {
			clearInterval(statsTimer);
			clearInterval(contentTimer);
			clearInterval(clockTimer);
		};
	});

	$effect(() => {
		selectedNodeId;
		void refreshNodeContent();
	});

	$effect(() => {
		const hasInternetWidget = widgetsReady && widgets.some((widget) => widget.type === 'internet-speed');
		const nodeId = selectedNodeId;
		if (!hasInternetWidget || !pageVisible || !nodeId || !selectedNodeConnected) return;

		void measureInternetSpeed(nodeId);
		const timer = setInterval(() => void measureInternetSpeed(nodeId), speedTestIntervalMs);
		return () => clearInterval(timer);
	});

	function appSubtitle(app: LaunchableApp): string {
		const value = `${app.name} ${app.container.image}`.toLowerCase();
		if (/plex|jellyfin|emby/.test(value)) return 'Media Server';
		if (/nextcloud|syncthing|file/.test(value)) return 'Files & Sync';
		if (/home.?assistant/.test(value)) return 'Smart Home';
		if (/portainer|dockge/.test(value)) return 'Container Mgmt';
		if (/adguard|pihole/.test(value)) return 'Network';
		if (/uptime|grafana|prometheus/.test(value)) return 'Monitoring';
		if (/torrent|qbittorrent|sabnzbd/.test(value)) return 'Download';
		return 'Docker App';
	}

	function openApp(app: LaunchableApp) {
		if (!app.port || app.container.state !== 'running') {
			setSection('containers');
			return;
		}
		window.open(`${window.location.protocol}//${window.location.hostname}:${app.port}`, '_blank', 'noopener,noreferrer');
	}

	function toggleAppMenu(event: MouseEvent, containerId: string) {
		event.stopPropagation();
		openAppMenuId = openAppMenuId === containerId ? null : containerId;
	}

	function openAppFromMenu(event: MouseEvent, app: LaunchableApp) {
		event.stopPropagation();
		openAppMenuId = null;
		openApp(app);
	}

	async function runAppAction(event: MouseEvent, app: LaunchableApp, action: ContainerAction) {
		event.stopPropagation();
		if (!selectedNodeId || appActionInFlight) return;
		openAppMenuId = null;
		appActionInFlight = { id: app.container.id, action };
		try {
			await containerAction(selectedNodeId, app.container.id, action);
			await refreshNodeContent();
			const verb = action === 'stop' ? 'stopped' : action === 'start' ? 'started' : 'restarted';
			toastSuccess(`${app.name} ${verb}`);
		} catch (err) {
			toastError(err instanceof Error ? err.message : `Could not ${action} ${app.name}`);
		} finally {
			appActionInFlight = null;
		}
	}

	function showAppDetails(event: MouseEvent) {
		event.stopPropagation();
		openAppMenuId = null;
		setSection('containers');
	}

	function appMenuKey(app: LaunchableApp): string {
		return `app:${app.id}`;
	}

	function shortcutMenuKey(shortcut: DashboardShortcut): string {
		return `shortcut:${shortcut.id}`;
	}

	function displayedAppIcon(app: LaunchableApp): string | undefined {
		return customAppIcons[app.id] || app.icon;
	}

	function faviconForUrl(value: string): string | undefined {
		try {
			return new URL('/favicon.ico', value).href;
		} catch {
			return undefined;
		}
	}

	function displayedShortcutIcon(shortcut: DashboardShortcut): string | undefined {
		return shortcut.icon || faviconForUrl(shortcut.url);
	}

	function shortcutDraftPreview(): string | undefined {
		if (shortcutDraftIcon.trim()) return shortcutDraftIcon.trim();
		try {
			return faviconForUrl(normalizeWebUrl(shortcutDraftUrl));
		} catch {
			return undefined;
		}
	}

	function normalizeWebUrl(value: string): string {
		const candidate = /^https?:\/\//i.test(value.trim()) ? value.trim() : `https://${value.trim()}`;
		const parsed = new URL(candidate);
		if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') throw new Error('Use an HTTP or HTTPS address.');
		return parsed.href;
	}

	function validIconValue(value: string): boolean {
		if (!value) return true;
		if (value.startsWith('data:image/')) return true;
		try {
			const parsed = new URL(value);
			return parsed.protocol === 'http:' || parsed.protocol === 'https:';
		} catch {
			return false;
		}
	}

	function normalizeShortcuts(value: unknown): DashboardShortcut[] {
		if (!Array.isArray(value)) return [];
		return value.flatMap((candidate): DashboardShortcut[] => {
			if (!candidate || typeof candidate !== 'object') return [];
			const shortcut = candidate as Partial<DashboardShortcut>;
			if (typeof shortcut.id !== 'string' || typeof shortcut.name !== 'string' || typeof shortcut.url !== 'string') return [];
			try {
				const url = normalizeWebUrl(shortcut.url);
				const icon = typeof shortcut.icon === 'string' && validIconValue(shortcut.icon) ? shortcut.icon : undefined;
				return [{ id: shortcut.id, name: shortcut.name.trim().slice(0, 60) || new URL(url).hostname, url, icon }];
			} catch {
				return [];
			}
		});
	}

	function normalizeIconMap(value: unknown): Record<string, string> {
		if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
		return Object.fromEntries(
			Object.entries(value)
				.filter(([key, icon]) => key && typeof icon === 'string' && validIconValue(icon))
				.slice(0, 100)
		) as Record<string, string>;
	}

	function normalizeAppOrder(value: unknown): string[] {
		if (!Array.isArray(value)) return [];
		return [...new Set(value.filter((id): id is string => typeof id === 'string' && id.length > 0))].slice(0, 200);
	}

	function reorderApp(sourceId: string, targetId: string) {
		if (sourceId === targetId) return;
		const currentIds = orderedLaunchableApps.map((app) => app.id);
		const sourceIndex = currentIds.indexOf(sourceId);
		const targetIndex = currentIds.indexOf(targetId);
		if (sourceIndex < 0 || targetIndex < 0) return;
		const reordered = [...currentIds];
		const [moved] = reordered.splice(sourceIndex, 1);
		reordered.splice(targetIndex, 0, moved);
		const unavailableIds = appOrder.filter((id) => !currentIds.includes(id));
		appOrder = [...reordered, ...unavailableIds];
	}

	function moveApp(appId: string, direction: -1 | 1) {
		const currentIds = orderedLaunchableApps.map((app) => app.id);
		const index = currentIds.indexOf(appId);
		const target = index + direction;
		if (index < 0 || target < 0 || target >= currentIds.length) return;
		reorderApp(appId, currentIds[target]);
		openAppMenuId = null;
	}

	function startAppDrag(event: DragEvent, appId: string) {
		draggedAppId = appId;
		appDropTargetId = null;
		if (event.dataTransfer) {
			event.dataTransfer.effectAllowed = 'move';
			event.dataTransfer.setData('text/plain', appId);
		}
	}

	function hoverAppDrop(event: DragEvent, appId: string) {
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		if (draggedAppId && draggedAppId !== appId) appDropTargetId = appId;
	}

	function dropApp(event: DragEvent, targetId: string) {
		event.preventDefault();
		const sourceId = draggedAppId || event.dataTransfer?.getData('text/plain');
		if (sourceId) reorderApp(sourceId, targetId);
		finishAppDrag();
	}

	function finishAppDrag() {
		draggedAppId = null;
		appDropTargetId = null;
	}

	function openShortcut(shortcut: DashboardShortcut) {
		window.open(shortcut.url, '_blank', 'noopener,noreferrer');
	}

	function openShortcutEditor(event?: MouseEvent, shortcut?: DashboardShortcut) {
		event?.stopPropagation();
		openAppMenuId = null;
		editingShortcutId = shortcut?.id ?? null;
		shortcutDraftName = shortcut?.name ?? '';
		shortcutDraftUrl = shortcut?.url ?? '';
		shortcutDraftIcon = shortcut?.icon ?? '';
		showShortcutModal = true;
	}

	function closeShortcutEditor() {
		showShortcutModal = false;
		editingShortcutId = null;
		shortcutDraftName = '';
		shortcutDraftUrl = '';
		shortcutDraftIcon = '';
		shortcutIconFileLoading = false;
	}

	function submitShortcut(event: SubmitEvent) {
		event.preventDefault();
		try {
			const url = normalizeWebUrl(shortcutDraftUrl);
			const icon = shortcutDraftIcon.trim();
			if (!validIconValue(icon)) throw new Error('The icon must be an HTTP(S) URL or an uploaded image.');
			const name = shortcutDraftName.trim().slice(0, 60) || new URL(url).hostname;
			if (editingShortcutId) {
				dashboardShortcuts = dashboardShortcuts.map((shortcut) => shortcut.id === editingShortcutId
					? { ...shortcut, name, url, icon: icon || undefined }
					: shortcut);
				toastSuccess(`${name} updated`);
			} else {
				const id = typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : `shortcut-${Date.now()}`;
				dashboardShortcuts = [{ id, name, url, icon: icon || undefined }, ...dashboardShortcuts];
				toastSuccess(`${name} added to the dashboard`);
			}
			closeShortcutEditor();
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Could not save shortcut');
		}
	}

	function removeShortcut(event: MouseEvent, shortcut: DashboardShortcut) {
		event.stopPropagation();
		openAppMenuId = null;
		if (!confirm(`Remove the ${shortcut.name} shortcut?`)) return;
		dashboardShortcuts = dashboardShortcuts.filter((candidate) => candidate.id !== shortcut.id);
		toastSuccess(`${shortcut.name} removed`);
	}

	function openAppIconEditor(event: MouseEvent, app: LaunchableApp) {
		event.stopPropagation();
		openAppMenuId = null;
		iconTarget = { kind: 'app', id: app.id, name: app.name, currentIcon: customAppIcons[app.id] ?? '' };
		iconDraft = customAppIcons[app.id] ?? '';
	}

	function openShortcutIconEditor(event: MouseEvent, shortcut: DashboardShortcut) {
		event.stopPropagation();
		openAppMenuId = null;
		iconTarget = { kind: 'shortcut', id: shortcut.id, name: shortcut.name, currentIcon: shortcut.icon ?? '' };
		iconDraft = shortcut.icon ?? '';
	}

	function closeIconEditor() {
		iconTarget = null;
		iconDraft = '';
		iconFileLoading = false;
	}

	function saveIcon(event: SubmitEvent) {
		event.preventDefault();
		if (!iconTarget) return;
		const icon = iconDraft.trim();
		if (!validIconValue(icon)) {
			toastError('The icon must be an HTTP(S) URL or an uploaded image.');
			return;
		}
		if (iconTarget.kind === 'app') {
			const next = { ...customAppIcons };
			if (icon) next[iconTarget.id] = icon;
			else delete next[iconTarget.id];
			customAppIcons = next;
		} else {
			dashboardShortcuts = dashboardShortcuts.map((shortcut) => shortcut.id === iconTarget?.id
				? { ...shortcut, icon: icon || undefined }
				: shortcut);
		}
		toastSuccess(icon ? `Icon updated for ${iconTarget.name}` : `Default icon restored for ${iconTarget.name}`);
		closeIconEditor();
	}

	async function handleIconFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		if (!file.type.startsWith('image/')) {
			toastError('Choose an image file.');
			return;
		}
		iconFileLoading = true;
		try {
			iconDraft = await resizeIconFile(file);
		} catch {
			toastError('Could not read that image.');
		} finally {
			iconFileLoading = false;
			input.value = '';
		}
	}

	async function handleShortcutIconFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		if (!file.type.startsWith('image/')) {
			toastError('Choose an image file.');
			return;
		}
		shortcutIconFileLoading = true;
		try {
			shortcutDraftIcon = await resizeIconFile(file);
		} catch {
			toastError('Could not read that image.');
		} finally {
			shortcutIconFileLoading = false;
			input.value = '';
		}
	}

	async function resizeIconFile(file: File): Promise<string> {
		const objectUrl = URL.createObjectURL(file);
		try {
			const image = new Image();
			await new Promise<void>((resolve, reject) => {
				image.onload = () => resolve();
				image.onerror = () => reject(new Error('Invalid image'));
				image.src = objectUrl;
			});
			const maxSize = 256;
			const scale = Math.min(1, maxSize / Math.max(image.naturalWidth, image.naturalHeight));
			const width = Math.max(1, Math.round(image.naturalWidth * scale));
			const height = Math.max(1, Math.round(image.naturalHeight * scale));
			const canvas = document.createElement('canvas');
			canvas.width = width;
			canvas.height = height;
			const context = canvas.getContext('2d');
			if (!context) throw new Error('Canvas unavailable');
			context.drawImage(image, 0, 0, width, height);
			return canvas.toDataURL('image/webp', 0.9);
		} finally {
			URL.revokeObjectURL(objectUrl);
		}
	}

	function relativeTime(value: string): string {
		const seconds = Math.max(0, Math.round((Date.now() - new Date(value).getTime()) / 1000));
		if (seconds < 60) return 'just now';
		const minutes = Math.round(seconds / 60);
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.round(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.round(hours / 24);
		return `${days}d ago`;
	}

	function clampPercent(value: number): number {
		return Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
	}

	function recordMetricSamples(nextNodes: Node[]) {
		let changed = false;
		const nextHistory = { ...metricHistory };
		for (const node of nextNodes) {
			if (!node.connected || !node.stats.timestamp) continue;
			const timestamp = new Date(node.stats.timestamp).getTime();
			if (!Number.isFinite(timestamp) || Math.abs(Date.now() - timestamp) >= 5000) continue;
			const history = nextHistory[node.id] ?? [];
			if (history.at(-1)?.timestamp === node.stats.timestamp) continue;

			nextHistory[node.id] = [...history, {
				timestamp: node.stats.timestamp,
				cpu: clampPercent(node.stats.cpuPercent),
				memory: node.stats.memTotalBytes
					? clampPercent((node.stats.memUsedBytes / node.stats.memTotalBytes) * 100)
					: 0,
				storage: node.stats.diskTotalBytes
					? clampPercent((node.stats.diskUsedBytes / node.stats.diskTotalBytes) * 100)
					: 0,
				receive: Math.max(0, node.stats.networkReceiveMbps ?? 0),
				transmit: Math.max(0, node.stats.networkTransmitMbps ?? 0)
			}].slice(-metricHistorySize);
			changed = true;
		}
		if (changed) metricHistory = nextHistory;
	}

	function sparklinePath(points: MetricPoint[], key: MetricKey): string {
		if (points.length < 2) return '';
		return points.map((point, index) => {
			const x = (index / (points.length - 1)) * 100;
			const y = 33 - (clampPercent(point[key]) / 100) * 31;
			return `${index === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`;
		}).join(' ');
	}

	function sparklineFill(path: string): string {
		return path ? `${path} L100 34 L0 34 Z` : '';
	}

	function formatNetworkRate(value: number): string {
		if (value >= 10) return value.toFixed(1);
		if (value >= 0.01) return value.toFixed(2);
		if (value > 0) return value.toFixed(3);
		return '0.00';
	}

	async function measureInternetSpeed(nodeId: string) {
		if (speedTestInFlightNodeId !== null) return;
		speedTestInFlightNodeId = nodeId;
		speedTestRunningNodeId = nodeId;
		speedTestError = null;
		try {
			const result = await runInternetSpeedTest(nodeId);
			speedTestResult = { nodeId, value: result };
		} catch (err) {
			speedTestError = {
				nodeId,
				message: err instanceof Error ? err.message : 'Internet speed test failed'
			};
		} finally {
			speedTestInFlightNodeId = null;
			speedTestRunningNodeId = null;
		}
	}

	function openSpotlight() {
		window.dispatchEvent(new CustomEvent('faroos:spotlight'));
	}

	function normalizeWidgets(value: unknown): DashboardWidget[] {
		if (!Array.isArray(value)) return defaultWidgets;
		return value.slice(0, 24).flatMap((candidate): DashboardWidget[] => {
			if (!candidate || typeof candidate !== 'object') return [];
			const widget = candidate as Partial<DashboardWidget>;
			if ((widget.type !== 'internet-speed' && widget.type !== 'html') || typeof widget.id !== 'string') return [];
			const accent = widgetAccents.includes(widget.accent as WidgetAccent)
				? (widget.accent as WidgetAccent)
				: '#3478f6';
			const fallbackTitle = widget.type === 'html' ? 'HTML Widget' : 'Live Internet';
			return [{
				id: widget.id,
				type: widget.type,
				title: typeof widget.title === 'string' && widget.title.trim()
					? (widget.title.trim() === 'Internet Speed' ? 'Live Internet' : widget.title.trim().slice(0, 40))
					: fallbackTitle,
				accent,
				html: widget.type === 'html'
					? (typeof widget.html === 'string' ? widget.html.slice(0, 100_000) : defaultHTMLWidget)
					: undefined
			}];
		});
	}

	function openWidgetEditor(widget?: DashboardWidget) {
		editingWidgetId = widget?.id ?? null;
		widgetDraftType = widget?.type ?? 'internet-speed';
		widgetDraftTitle = widget?.title ?? 'Live Internet';
		widgetDraftAccent = widget?.accent ?? '#3478f6';
		widgetDraftHTML = widget?.html ?? defaultHTMLWidget;
		showWidgetModal = true;
	}

	function selectWidgetType(type: WidgetType) {
		if (widgetDraftType === type) return;
		const oldDefault = widgetDraftType === 'html' ? 'HTML Widget' : 'Live Internet';
		widgetDraftType = type;
		if (!widgetDraftTitle.trim() || widgetDraftTitle === oldDefault) {
			widgetDraftTitle = type === 'html' ? 'HTML Widget' : 'Live Internet';
		}
	}

	function closeWidgetEditor() {
		showWidgetModal = false;
		editingWidgetId = null;
	}

	function submitWidget(event: SubmitEvent) {
		event.preventDefault();
		if (!editingWidgetId && widgets.length >= 24) {
			toastError('The dashboard supports up to 24 widgets.');
			return;
		}
		const title = widgetDraftTitle.trim().slice(0, 40) || (widgetDraftType === 'html' ? 'HTML Widget' : 'Live Internet');
		const nextWidget: DashboardWidget = {
			id: editingWidgetId ?? (typeof crypto !== 'undefined' && crypto.randomUUID
				? crypto.randomUUID()
				: `widget-${Date.now()}`),
			type: widgetDraftType,
			title,
			accent: widgetDraftAccent,
			html: widgetDraftType === 'html' ? widgetDraftHTML.slice(0, 100_000) : undefined
		};
		if (editingWidgetId) {
			widgets = widgets.map((widget) => widget.id === editingWidgetId
				? nextWidget
				: widget);
		} else {
			widgets = [...widgets, nextWidget];
		}
		closeWidgetEditor();
	}

	function removeWidget(id: string) {
		widgets = widgets.filter((widget) => widget.id !== id);
	}

	function moveWidget(index: number, direction: -1 | 1) {
		const target = index + direction;
		if (target < 0 || target >= widgets.length) return;
		const reordered = [...widgets];
		[reordered[index], reordered[target]] = [reordered[target], reordered[index]];
		widgets = reordered;
	}

	async function submitPairing(event: SubmitEvent) {
		event.preventDefault();
		if (!newNodeName.trim()) return;
		if (!relayEnabled && !resolvedPanelAddress()) {
			toastError('Introduce una dirección pública HTTP o HTTPS válida para el panel');
			return;
		}
		pairing = true;
		try {
			pairingResult = await createPairing(newNodeName.trim());
			await refreshNodes();
		} catch (err) {
			toastError(err instanceof Error ? err.message : 'Could not create the server');
		} finally {
			pairing = false;
		}
	}

	function closePairing() {
		showPairModal = false;
		newNodeName = '';
		pairingResult = null;
	}

	function openPairing() {
		profileOpen = false;
		newNodeName = '';
		pairingResult = null;
		panelAddress = typeof window !== 'undefined' ? window.location.origin : '';
		showPairModal = true;
	}

	async function copyPairing() {
		if (!pairingResult) return;
		const command = installCommand(pairingResult);
		try {
			let copied = false;
			if (navigator.clipboard?.writeText) {
				try {
					await navigator.clipboard.writeText(command);
					copied = true;
				} catch {
					// LAN panels commonly use HTTP, where Clipboard can be present but denied.
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
				if (!copied) throw new Error('Clipboard unavailable');
			}
			toastSuccess('Comando de instalación copiado');
		} catch {
			toastError('No se pudo copiar; selecciona el comando manualmente');
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

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape') return;
		profileOpen = false;
		openAppMenuId = null;
		if (showPairModal) closePairing();
		if (showWidgetModal) closeWidgetEditor();
		if (showShortcutModal) closeShortcutEditor();
		if (iconTarget) closeIconEditor();
	}
</script>

<svelte:window onkeydown={handleKeydown} onclick={() => (openAppMenuId = null)} />

<div class="dashboard-shell section-enter">
	<header class="dashboard-topbar">
		<button class="wordmark" type="button" onclick={() => setSection('dashboard')}>FaroOS</button>
		<div class="topbar-actions">
			<button type="button" class="search-bar" onclick={openSpotlight}>
				<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6.5" /><path d="m16 16 4 4" /></svg>
				<span>Search apps, files, settings...</span><kbd>⌘ K</kbd>
			</button>
			<button type="button" class="round-action" aria-label="Notifications" title="No new notifications">
				<svg viewBox="0 0 24 24"><path d="M18 9a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4" /></svg>
			</button>
			<div class="profile-wrap">
				<button type="button" class="profile-button" aria-label="Open profile menu" onclick={() => (profileOpen = !profileOpen)}>G</button>
				{#if profileOpen}
					<div class="profile-menu">
						<div class="profile-copy"><strong>Gonzalo</strong><span>Administrator</span></div>
						<button type="button" onclick={openPairing}><span>＋</span>Add server</button>
						<button type="button" onclick={() => { profileOpen = false; setSection('settings'); }}><span>⚙</span>Settings</button>
					</div>
				{/if}
			</div>
		</div>
	</header>

	<main class="dashboard-content">
		<section class="welcome-copy">
			<div>
				<p class="welcome-kicker">Control center · {connectedNodes.length} {connectedNodes.length === 1 ? 'server online' : 'servers online'}</p>
				<h1>{greeting}, <strong>Gonzalo.</strong></h1>
				<p>Everything in your infrastructure, live and in one place.</p>
			</div>
			<div class="welcome-actions">
				<span class="fleet-online"><i></i>{connectedNodes.length ? 'Systems operational' : 'Waiting for servers'}</span>
				<button type="button" onclick={openPairing}><span>＋</span>Add server</button>
			</div>
		</section>

		<section class="overview-strip" aria-label="Infrastructure overview">
			<button type="button" class="overview-tile servers-tile" onclick={() => setSection('servers')}>
				<span class="overview-icon"><svg viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="6" rx="2" /><rect x="4" y="14" width="16" height="6" rx="2" /><path d="M8 7h.01M8 17h.01M16 7h1M16 17h1" /></svg></span>
				<span class="overview-copy"><small>Servers</small><strong>{connectedNodes.length}<em> / {nodes.length || 0}</em></strong><span>{connectedNodes.length === nodes.length && nodes.length ? 'All connected' : 'Fleet status'}</span></span>
				<i>›</i>
			</button>
			<button type="button" class="overview-tile cpu-tile" onclick={() => setSection('servers')}>
				<span class="overview-icon"><svg viewBox="0 0 24 24"><rect x="7" y="7" width="10" height="10" rx="2" /><path d="M9 2v3m6-3v3M9 19v3m6-3v3M2 9h3m-3 6h3m14-6h3m-3 6h3" /></svg></span>
				<span class="overview-copy"><small>Processor</small><strong>{statsAreLive ? `${cpuPercent.toFixed(0)}%` : '—'}</strong><span>{selectedNode?.name ?? 'No server selected'}</span></span>
			</button>
			<button type="button" class="overview-tile memory-tile" onclick={() => setSection('servers')}>
				<span class="overview-icon"><svg viewBox="0 0 24 24"><path d="M4 7h16v10H4zM7 10v4m3-4v4m3-4v4m3-4v4M7 4v3m3-3v3m3-3v3m3-3v3M7 17v3m3-3v3m3-3v3m3-3v3" /></svg></span>
				<span class="overview-copy"><small>Memory</small><strong>{selectedNode ? formatMemory(selectedNode.stats.memTotalBytes) : '—'}</strong><span>{statsAreLive ? `${memoryPercent.toFixed(1)}% in use` : 'Waiting for live data'}</span></span>
			</button>
			<button type="button" class="overview-tile storage-tile" onclick={() => setSection('storage')}>
				<span class="overview-icon"><svg viewBox="0 0 24 24"><path d="M4 7c0-2 3.6-3 8-3s8 1 8 3-3.6 3-8 3-8-1-8-3Zm0 0v5c0 2 3.6 3 8 3s8-1 8-3V7M4 12v5c0 2 3.6 3 8 3s8-1 8-3v-5" /></svg></span>
				<span class="overview-copy"><small>Total storage</small><strong>{formatStorage(fleetStorage)}</strong><span>Across connected disks</span></span>
				<i>›</i>
			</button>
		</section>

		{#if loadError}<div class="dashboard-error">{loadError}</div>{/if}

		<div class="dashboard-grid">
			<div class="dashboard-main">
				{#if loading}
					<div class="server-card skeleton"></div>
				{:else if !selectedNode}
					<button type="button" class="server-card empty-server" onclick={openPairing}>
						<span>＋</span><strong>Add your first server</strong><small>One place for apps, files and system health.</small>
					</button>
				{:else}
					<section class="server-card">
						<div class="server-device" aria-hidden="true">
							<img src={serverDeviceImage} alt="" />
						</div>

						<div class="server-identity">
							{#if nodes.length > 1}
								<select bind:value={selectedNodeId} aria-label="Select server">{#each nodes as node (node.id)}<option value={node.id}>{node.name}</option>{/each}</select>
							{:else}<h2>{selectedNode.name || 'FaroOS Host'}</h2>{/if}
							<p class="hardware-name">Linux home server</p>
							<p class="uptime">Uptime: {formatUptime(selectedNode.stats.uptimeSeconds)}</p>
							<div class="status-chips">
								<span><b>System</b><em class:healthy={statsAreLive}>{statsAreLive ? 'Live' : selectedNode.connected ? 'Waiting' : 'Offline'}</em></span>
								<span><b>Docker</b><em class:healthy={runningContainers > 0}>{runningContainers > 0 ? 'Running' : 'Stopped'}</em></span>
								<span><b>Apps</b><em class="blue-status">{runningContainers} active</em></span>
							</div>
						</div>

						<div class="cpu-block">
							<div class="cpu-ring" style={`--progress:${cpuPercent * 3.6}deg`}><div><strong>{statsAreLive ? `${cpuPercent.toFixed(0)}%` : '—'}</strong><span>CPU Usage</span></div></div>
						</div>

					</section>
				{/if}

				<section class="installed-section">
					<div class="section-heading"><h2>Installed Apps</h2><div class="section-heading-actions"><button type="button" onclick={() => openShortcutEditor()}>＋ Shortcut</button><button type="button" onclick={() => setSection('apps')}>View All <span>›</span></button></div></div>
					<div class="apps-grid">
						{#each visibleDashboardShortcuts as shortcut (shortcut.id)}
							<article class="app-card" class:menu-open={openAppMenuId === shortcutMenuKey(shortcut)}>
								<button type="button" class="app-card-main" onclick={() => openShortcut(shortcut)}>
									<AppIcon name={shortcut.name} icon={displayedShortcutIcon(shortcut)} size={44} />
									<span class="app-copy"><strong>{shortcut.name}</strong><small>Web shortcut</small><em class="running"><i></i>Ready</em></span>
								</button>
								<button type="button" class="app-menu" aria-label={`Actions for ${shortcut.name}`} aria-expanded={openAppMenuId === shortcutMenuKey(shortcut)} onclick={(event) => toggleAppMenu(event, shortcutMenuKey(shortcut))}>•••</button>
								{#if openAppMenuId === shortcutMenuKey(shortcut)}
									<div class="app-context-menu" role="menu" tabindex="-1">
										<button type="button" role="menuitem" onclick={(event) => { event.stopPropagation(); openAppMenuId = null; openShortcut(shortcut); }}>Open shortcut</button>
										<button type="button" role="menuitem" onclick={(event) => openShortcutEditor(event, shortcut)}>Edit shortcut</button>
										<button type="button" role="menuitem" onclick={(event) => openShortcutIconEditor(event, shortcut)}>Change icon</button>
										<button type="button" role="menuitem" onclick={(event) => openShortcutEditor(event)}>Add shortcut</button>
										<button type="button" class="danger" role="menuitem" onclick={(event) => removeShortcut(event, shortcut)}>Remove</button>
									</div>
								{/if}
							</article>
						{/each}
						{#each visibleDashboardApps as app (app.id)}
							<article
								class="app-card"
								class:menu-open={openAppMenuId === appMenuKey(app)}
								class:dragging={draggedAppId === app.id}
								class:drop-target={appDropTargetId === app.id}
								draggable="true"
								title="Drag to reorder"
								ondragstart={(event) => startAppDrag(event, app.id)}
								ondragover={(event) => hoverAppDrop(event, app.id)}
								ondrop={(event) => dropApp(event, app.id)}
								ondragend={finishAppDrag}
							>
								<button type="button" class="app-card-main" onclick={() => openApp(app)}>
									<AppIcon name={app.name} icon={displayedAppIcon(app)} size={44} />
									<span class="app-copy"><strong>{app.name}</strong><small>{appSubtitle(app)}</small><em class:running={app.container.state === 'running'}><i></i>{app.container.state === 'running' ? 'Running' : 'Stopped'}</em></span>
								</button>
								<button type="button" class="app-menu" aria-label={`Actions for ${app.name}`} aria-expanded={openAppMenuId === appMenuKey(app)} onclick={(event) => toggleAppMenu(event, appMenuKey(app))}>•••</button>
								{#if openAppMenuId === appMenuKey(app)}
									<div class="app-context-menu" role="menu" tabindex="-1">
										{#if app.port && app.container.state === 'running'}<button type="button" role="menuitem" onclick={(event) => openAppFromMenu(event, app)}>Open app</button>{/if}
										{#if app.container.state === 'running'}
											<button type="button" role="menuitem" disabled={Boolean(appActionInFlight)} onclick={(event) => runAppAction(event, app, 'restart')}>Restart</button>
											<button type="button" role="menuitem" disabled={Boolean(appActionInFlight)} onclick={(event) => runAppAction(event, app, 'stop')}>Stop</button>
										{:else}
											<button type="button" role="menuitem" disabled={Boolean(appActionInFlight)} onclick={(event) => runAppAction(event, app, 'start')}>Start</button>
										{/if}
										<button type="button" role="menuitem" onclick={showAppDetails}>View container</button>
										<button type="button" role="menuitem" disabled={orderedLaunchableApps.findIndex((candidate) => candidate.id === app.id) === 0} onclick={(event) => { event.stopPropagation(); moveApp(app.id, -1); }}>Move left</button>
										<button type="button" role="menuitem" disabled={orderedLaunchableApps.findIndex((candidate) => candidate.id === app.id) === orderedLaunchableApps.length - 1} onclick={(event) => { event.stopPropagation(); moveApp(app.id, 1); }}>Move right</button>
										<button type="button" role="menuitem" onclick={(event) => openAppIconEditor(event, app)}>Change icon</button>
										<button type="button" role="menuitem" onclick={(event) => openShortcutEditor(event)}>Add shortcut</button>
									</div>
								{/if}
							</article>
						{/each}
						{#each Array(emptyDashboardAppSlots) as _, index (index)}
							<button type="button" class="app-card app-placeholder" onclick={() => setSection('apps')}><span>＋</span><strong>Add app</strong><small>Browse the store</small></button>
						{/each}
					</div>
				</section>

				<section class="recent-card">
					<div class="section-heading"><h2>Recent Files</h2><button type="button" onclick={() => setSection('files')}>Open Files <span>›</span></button></div>
					<div class="recent-list">
						{#each visibleRecentEntries as entry (entry.name)}
							<button type="button" onclick={() => setSection('files')}>
								<span class:folder={entry.isDir} class="file-icon">
									{#if entry.isDir}<img src={folderIcon} alt="" />{:else}<svg viewBox="0 0 28 32"><path d="M5 1h12l7 7v23H5V1Z" /><path d="M17 1v8h7" /></svg>{/if}
								</span>
								<strong>{entry.name}</strong><small>{relativeTime(entry.modTime)}</small>
							</button>
						{/each}
						{#if visibleRecentEntries.length === 0}<button type="button" class="recent-empty" onclick={() => setSection('files')}><span class="file-icon folder"><img src={folderIcon} alt="" /></span><strong>Browse files</strong><small>Open the server</small></button>{/if}
					</div>
				</section>
			</div>

			<aside class="dashboard-side">
				<section class="side-card system-status">
					<div class="side-heading"><h2>System Status</h2><svg viewBox="0 0 24 24"><path d="M3 13h4l2-7 4 13 3-9 2 3h3" /></svg></div>
					<div class="metric-row"><span>CPU</span><strong>{statsAreLive ? `${cpuPercent.toFixed(0)}%` : '—'}</strong><svg viewBox="0 0 100 34" class="spark cpu" aria-label="Live CPU history"><path class="fill" d={sparklineFill(cpuPath)} /><path d={cpuPath} /></svg></div>
					<div class="metric-row"><span>RAM</span><strong>{statsAreLive ? `${memoryPercent.toFixed(0)}%` : '—'}</strong><svg viewBox="0 0 100 34" class="spark ram" aria-label="Live memory history"><path class="fill" d={sparklineFill(memoryPath)} /><path d={memoryPath} /></svg></div>
					<div class="metric-row"><span>Storage</span><strong>{statsAreLive ? `${storagePercent.toFixed(0)}%` : '—'}</strong><svg viewBox="0 0 100 34" class="spark storage" aria-label="Live storage history"><path class="fill" d={sparklineFill(storagePath)} /><path d={storagePath} /></svg></div>
					<div class="network-row"><span>Docker</span><div><b>↑</b> {runningContainers} running</div><div><b>↓</b> {stoppedContainers} stopped</div></div>
				</section>

				<section class="side-card quick-actions">
					<h2>Quick Actions</h2>
					<button type="button" onclick={() => setSection('files')}><svg viewBox="0 0 24 24"><path d="M8 17H6a4 4 0 0 1-.5-8A6 6 0 0 1 17 8a4.5 4.5 0 0 1 .5 9H16M12 12v9m-3-6 3-3 3 3" /></svg><span>Upload Files</span><i>›</i></button>
					<button type="button" onclick={() => setSection('apps')}><svg viewBox="0 0 24 24"><path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Zm0 9 8-4.5M12 12 4 7.5M12 12v9" /></svg><span>Create Container</span><i>›</i></button>
					<button type="button" onclick={() => setSection('terminal')}><svg viewBox="0 0 24 24"><path d="m5 7 5 5-5 5m8 0h6" /></svg><span>Terminal</span><i>›</i></button>
					<button type="button" onclick={() => setSection('settings')}><svg viewBox="0 0 24 24"><path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Zm7-3.5 2-1-2-3.5-2.2.7a8 8 0 0 0-2-1.2L14.3 5h-4.6L9 7a8 8 0 0 0-2 1.2l-2-.7L3 11l2 1a8 8 0 0 0 0 2l-2 1 2 3.5 2-.7A8 8 0 0 0 9 19l.7 2h4.6l.7-2a8 8 0 0 0 2-1.2l2 .7L21 15l-2-1a8 8 0 0 0 0-2Z" /></svg><span>System Settings</span><i>›</i></button>
				</section>
			</aside>
		</div>

		<section class="widgets-section" aria-labelledby="widgets-heading">
			<div class="widgets-heading">
				<div><p>Personal dashboard</p><h2 id="widgets-heading">My Widgets</h2></div>
				<button type="button" class="add-widget-button" onclick={() => openWidgetEditor()}><span>＋</span>Add widget</button>
			</div>
			{#if widgets.length === 0}
				<button type="button" class="widgets-empty" onclick={() => openWidgetEditor()}><span>＋</span><strong>Create your first widget</strong><small>Choose a data source, name and accent color.</small></button>
			{:else}
				<div class="widgets-grid">
					{#each widgets as widget, index (widget.id)}
						<article class="custom-widget" class:speed-widget={widget.type === 'internet-speed'} class:html-widget={widget.type === 'html'} style={`--widget-accent:${widget.accent}`}>
							<header>
								<div class="widget-title"><span><svg viewBox="0 0 24 24">{#if widget.type === 'html'}<path d="m8 7-5 5 5 5m8-10 5 5-5 5M14 4l-4 16" />{:else}<path d="M4 18a8 8 0 1 1 16 0M12 18l4-7M7 15h.01M12 11h.01M17 15h.01" />{/if}</svg></span><div><strong>{widget.title}</strong><small>{widget.type === 'html' ? 'Sandboxed HTML' : (selectedNode?.name ?? 'No server selected')}</small></div></div>
								<div class="widget-controls">
									<button type="button" aria-label="Move widget left" title="Move left" disabled={index === 0} onclick={() => moveWidget(index, -1)}>←</button>
									<button type="button" aria-label="Move widget right" title="Move right" disabled={index === widgets.length - 1} onclick={() => moveWidget(index, 1)}>→</button>
									<button type="button" aria-label="Edit widget" title="Edit" onclick={() => openWidgetEditor(widget)}>✎</button>
									<button type="button" class="remove-widget" aria-label="Remove widget" title="Remove" onclick={() => removeWidget(widget.id)}>×</button>
								</div>
							</header>

							{#if widget.type === 'html'}
								<div class="html-widget-frame">
									<iframe title={widget.title} srcdoc={widget.html ?? ''} sandbox="allow-scripts" referrerpolicy="no-referrer"></iframe>
								</div>
								<footer><span>Isolated from FaroOS and its administrator session</span><span class="html-indicator"><i></i>HTML</span></footer>
							{:else if selectedSpeedTest}
								<div class="speed-result">
									<div class="download-speed"><span>↓ Download</span><strong>{formatNetworkRate(selectedSpeedTest.downloadMbps)}</strong><small>Mbps</small></div>
									<div class="speed-secondary"><div><span>↑ Upload</span><strong>{formatNetworkRate(selectedSpeedTest.uploadMbps)} <small>Mbps</small></strong></div><div><span>Latency</span><strong>{selectedSpeedTest.latencyMs.toFixed(0)} <small>ms</small></strong></div></div>
								</div>
							{:else}
								<div class="speed-placeholder"><span class="signal"><i></i><i></i><i></i><i></i></span><div><strong>{speedTestRunning ? 'Measuring Internet speed…' : selectedSpeedTestError ? 'Measurement failed' : selectedNode?.connected ? 'Preparing real measurement…' : 'Server offline'}</strong><small>{selectedSpeedTestError ?? 'Download, upload and latency are tested only while this widget is on the visible dashboard.'}</small></div></div>
							{/if}

							{#if widget.type === 'internet-speed'}
								<footer>
									<span>{speedTestRunning ? 'Testing with Ookla…' : selectedSpeedTest ? `${selectedSpeedTest.provider}${selectedSpeedTest.server ? ` · ${selectedSpeedTest.server}` : ''} · measured ${relativeTime(selectedSpeedTest.testedAt)}` : selectedSpeedTestError ? 'Retrying in 30 seconds' : 'Waiting for the first Ookla measurement'}</span>
									<span class:active={Boolean(selectedSpeedTest) || speedTestRunning} class="live-indicator"><i></i>{speedTestRunning ? 'TESTING' : selectedNode?.connected ? 'EVERY 30S' : 'OFFLINE'}</span>
								</footer>
							{/if}
						</article>
					{/each}
				</div>
			{/if}
		</section>
	</main>
</div>

{#if showPairModal}
	<div class="modal-layer">
		<button type="button" aria-label="Close" class="modal-backdrop" onclick={closePairing}></button>
		<div class="pair-modal" class:command-modal={Boolean(pairingResult)} role="dialog" aria-modal="true">
			{#if !pairingResult}
				<span class="pair-icon">＋</span><p class="modal-kicker">Añadir servidor</p><h2>Prepara el comando</h2>
				<p>Elige un nombre. FaroOS generará un único comando con la autenticación incluida.</p>
				<form onsubmit={submitPairing}><label for="server-name">Nombre del servidor</label><input id="server-name" bind:value={newNodeName} placeholder="Servidor principal" />{#if relayEnabled}<aside class="relay-ready"><strong>{p2pEnabled ? 'Conexión directa P2P activa' : 'FaroOS Relay activo'}</strong><small>{p2pEnabled ? 'FaroOS coordina el encuentro y luego el tráfico pasa directamente entre servidores cuando la red lo permite.' : 'Funcionará desde cualquier red, sin VPN ni puertos abiertos.'}</small></aside>{:else}<label for="dashboard-panel-address">Dirección accesible desde ese servidor</label><input id="dashboard-panel-address" type="url" bind:value={panelAddress} placeholder="https://panel.tudominio.com" /><small>Para otra red, usa un dominio HTTPS público o la dirección de tu VPN.</small>{/if}<div><button type="button" onclick={closePairing}>Cancelar</button><button type="submit" class="modal-primary" disabled={pairing}>{pairing ? 'Generando…' : 'Generar comando'}</button></div></form>
			{:else}
				<span class="pair-icon success">1</span><p class="modal-kicker">Un comando · Linux</p><h2>Copia y ejecútalo en {pairingResult.name}</h2><p>No tienes que introducir identificadores ni contraseñas después. El comando lleva el emparejamiento privado y completa toda la instalación.</p>
				<div class="pair-install-command"><span aria-hidden="true">$</span><code>{installCommand(pairingResult)}</code><button type="button" class="modal-primary" onclick={copyPairing}>Copiar comando</button></div>
				<div class="pair-automation" aria-label="Acciones automáticas"><span>✓ Instala Docker</span><span>✓ Instala el agente</span><span>✓ Autentica</span><span>✓ Inicia el servicio</span></div>
				<div class:connected={pairingConnected} class="pair-connection" aria-live="polite"><i></i><span><strong>{pairingConnected ? 'Servidor autenticado y conectado' : 'Esperando la primera conexión…'}</strong><small>{pairingConnected ? 'Ya puedes administrarlo desde FaroOS.' : 'Pega el comando en una terminal del nuevo servidor.'}</small></span></div>
				<div class="modal-footer"><button type="button" onclick={closePairing}>{pairingConnected ? 'Terminar' : 'Cerrar'}</button></div>
			{/if}
		</div>
	</div>
{/if}

{#if showWidgetModal}
	<div class="modal-layer">
		<button type="button" aria-label="Close widget editor" class="modal-backdrop" onclick={closeWidgetEditor}></button>
		<div class="widget-builder" class:html-editor={widgetDraftType === 'html'} role="dialog" aria-modal="true" aria-labelledby="widget-builder-title">
			<header><div><p>Dashboard widget</p><h2 id="widget-builder-title">{editingWidgetId ? 'Customize widget' : 'Add a widget'}</h2></div><button type="button" aria-label="Close" onclick={closeWidgetEditor}>×</button></header>
			<form onsubmit={submitWidget}>
				<p class="field-label widget-type-label">Widget type</p>
				<div class="widget-type-grid">
					<button type="button" class="widget-type" class:selected={widgetDraftType === 'internet-speed'} onclick={() => selectWidgetType('internet-speed')}><span><svg viewBox="0 0 24 24"><path d="M4 18a8 8 0 1 1 16 0M12 18l4-7" /></svg></span><div><strong>Live Internet · Ookla</strong><small>Real download, upload and latency measurement.</small></div><i>{widgetDraftType === 'internet-speed' ? '✓' : ''}</i></button>
					<button type="button" class="widget-type" class:selected={widgetDraftType === 'html'} onclick={() => selectWidgetType('html')}><span><svg viewBox="0 0 24 24"><path d="m8 7-5 5 5 5m8-10 5 5-5 5M14 4l-4 16" /></svg></span><div><strong>HTML widget</strong><small>Custom HTML, CSS and JavaScript in an isolated frame.</small></div><i>{widgetDraftType === 'html' ? '✓' : ''}</i></button>
				</div>
				<label for="widget-name">Widget name</label>
				<input id="widget-name" bind:value={widgetDraftTitle} maxlength="40" placeholder="Live Internet" />
				{#if widgetDraftType === 'html'}
					<label for="widget-html">HTML, CSS and JavaScript</label>
					<textarea id="widget-html" bind:value={widgetDraftHTML} maxlength="100000" spellcheck="false" aria-describedby="widget-html-help"></textarea>
					<p id="widget-html-help" class="html-widget-help">Runs with scripts enabled inside a sandbox, without access to FaroOS cookies, session, parent page or top-level navigation.</p>
				{/if}
				<p class="field-label">Accent color</p>
				<div class="color-picker" aria-label="Accent color">{#each widgetAccents as accent}<button type="button" aria-label={`Use ${accent}`} class:active={widgetDraftAccent === accent} style={`--swatch:${accent}`} onclick={() => (widgetDraftAccent = accent)}><span></span></button>{/each}</div>
				<div class="widget-builder-actions"><button type="button" onclick={closeWidgetEditor}>Cancel</button><button type="submit" class="modal-primary">{editingWidgetId ? 'Save changes' : 'Add widget'}</button></div>
			</form>
		</div>
	</div>
{/if}

{#if showShortcutModal}
	<div class="modal-layer">
		<button type="button" aria-label="Close shortcut editor" class="modal-backdrop" onclick={closeShortcutEditor}></button>
		<div class="widget-builder customization-builder" role="dialog" aria-modal="true" aria-labelledby="shortcut-builder-title">
			<header><div><p>Dashboard shortcut</p><h2 id="shortcut-builder-title">{editingShortcutId ? 'Edit shortcut' : 'Add a shortcut'}</h2></div><button type="button" aria-label="Close" onclick={closeShortcutEditor}>×</button></header>
			<form onsubmit={submitShortcut}>
				<div class="customization-preview"><AppIcon name={shortcutDraftName || 'Shortcut'} icon={shortcutDraftPreview()} size={54} /><span><strong>{shortcutDraftName.trim() || 'New shortcut'}</strong><small>{shortcutDraftIcon.trim() ? 'Custom icon' : 'Favicon detected automatically'}</small></span></div>
				<label for="shortcut-url">Website URL</label>
				<input id="shortcut-url" bind:value={shortcutDraftUrl} required placeholder="https://example.com" spellcheck="false" />
				<label for="shortcut-name">Name <small>optional</small></label>
				<input id="shortcut-name" bind:value={shortcutDraftName} maxlength="60" placeholder="Uses the website hostname if empty" />
				<label for="shortcut-icon">Custom icon URL <small>optional</small></label>
				<input id="shortcut-icon" bind:value={shortcutDraftIcon} placeholder="Leave empty to use the website favicon" spellcheck="false" />
				<label class="icon-upload"><input type="file" accept="image/*" onchange={handleShortcutIconFile} /><span>{shortcutIconFileLoading ? 'Preparing image…' : 'Upload an icon instead'}</span></label>
				<p class="customization-help">Without a custom icon, FaroOS requests <code>/favicon.ico</code> directly from that website.</p>
				<div class="widget-builder-actions"><button type="button" onclick={closeShortcutEditor}>Cancel</button><button type="submit" class="modal-primary">{editingShortcutId ? 'Save changes' : 'Add shortcut'}</button></div>
			</form>
		</div>
	</div>
{/if}

{#if iconTarget}
	<div class="modal-layer">
		<button type="button" aria-label="Close icon editor" class="modal-backdrop" onclick={closeIconEditor}></button>
		<div class="widget-builder customization-builder" role="dialog" aria-modal="true" aria-labelledby="icon-builder-title">
			<header><div><p>Custom icon</p><h2 id="icon-builder-title">Change {iconTarget.name}</h2></div><button type="button" aria-label="Close" onclick={closeIconEditor}>×</button></header>
			<form onsubmit={saveIcon}>
				<div class="customization-preview"><AppIcon name={iconTarget.name} icon={iconDraft || undefined} size={58} /><span><strong>Icon preview</strong><small>{iconDraft ? 'Custom icon selected' : 'Default icon will be used'}</small></span></div>
				<label for="custom-icon-url">Icon URL</label>
				<input id="custom-icon-url" bind:value={iconDraft} placeholder="https://example.com/icon.png" spellcheck="false" />
				<label class="icon-upload"><input type="file" accept="image/*" onchange={handleIconFile} /><span>{iconFileLoading ? 'Preparing image…' : 'Upload PNG, JPG, WebP or SVG'}</span></label>
				<p class="customization-help">Uploaded images are reduced to 256 px and saved only with this dashboard configuration.</p>
				<div class="widget-builder-actions"><button type="button" onclick={() => (iconDraft = '')}>Use default</button><button type="button" onclick={closeIconEditor}>Cancel</button><button type="submit" class="modal-primary">Save icon</button></div>
			</form>
		</div>
	</div>
{/if}

<style>
	:global(:root) { --dashboard-blue: var(--accent); --dashboard-green: var(--success); --dashboard-purple: #7857d9; }
	.dashboard-shell { min-height: 100%; padding-bottom: 132px; color: #1d1f24; color-scheme: light; background: radial-gradient(circle at 50% -10%, #fff 0, transparent 34%), linear-gradient(145deg, #fafbfc 0%, #f5f6f8 58%, #f9fafb 100%); }
	.dashboard-topbar { width: min(1480px, 100%); height: 92px; display: flex; align-items: center; justify-content: space-between; gap: 32px; margin-inline: auto; padding: 22px 25px; }
	.wordmark { border: 0; background: transparent; color: #111318; font-size: 28px; font-weight: 730; letter-spacing: -.045em; }
	.topbar-actions { display: flex; align-items: center; gap: 12px; }
	.search-bar { width: min(465px, 39vw); height: 45px; display: flex; align-items: center; gap: 12px; padding: 0 15px; border: 1px solid rgba(19, 25, 35, .065); border-radius: 16px; color: #747a84; background: rgba(255,255,255,.84); box-shadow: 0 9px 28px rgba(22,28,38,.04), 0 2px 6px rgba(22,28,38,.025); backdrop-filter: blur(22px); text-align: left; }
	.search-bar svg { width: 19px; height: 19px; fill: none; stroke: #30343a; stroke-width: 1.8; }
	.search-bar span { flex: 1; font-size: 13px; }
	.search-bar kbd { padding: 3px 6px; border: 1px solid #e4e6e9; border-radius: 6px; color: #a1a6ae; background: #f8f9fa; font-size: 9px; }
	.round-action, .profile-button { width: 45px; height: 45px; display: grid; place-items: center; border: 1px solid transparent; border-radius: 14px; background: transparent; color: #1b1e23; }
	.round-action:hover { background: rgba(255,255,255,.8); border-color: rgba(19,25,35,.06); }
	.round-action svg { width: 20px; height: 20px; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
	.profile-wrap { position: relative; }
	.profile-button { border-color: rgba(19,25,35,.055); background: #eef0f2; font-size: 15px; font-weight: 650; box-shadow: inset 0 1px 0 rgba(255,255,255,.9); }
	.profile-menu { position: absolute; z-index: 20; top: 54px; right: 0; width: 210px; padding: 8px; border: 1px solid rgba(16,24,40,.08); border-radius: 18px; background: rgba(255,255,255,.94); box-shadow: 0 24px 60px rgba(15,23,42,.14); backdrop-filter: blur(30px); }
	.profile-copy { display: flex; flex-direction: column; padding: 10px 11px 12px; border-bottom: 1px solid #eceef0; }
	.profile-copy strong { font-size: 13px; }.profile-copy span { margin-top: 2px; color: #9297a0; font-size: 11px; }
	.profile-menu button { width: 100%; display: flex; align-items: center; gap: 9px; padding: 9px 10px; border: 0; border-radius: 10px; background: transparent; color: #30343a; font-size: 12px; text-align: left; }.profile-menu button:hover { background: #f4f5f7; }
	.dashboard-content { width: min(1228px, calc(100% - 48px)); margin-inline: auto; }
	.welcome-copy { margin: 6px 0 23px; }.welcome-copy h1 { margin: 0; font-size: 26px; font-weight: 440; letter-spacing: -.025em; }.welcome-copy h1 strong { font-weight: 720; }.welcome-copy p { margin: 7px 0 0; color: #777d86; font-size: 15px; }
	.dashboard-error { margin-bottom: 14px; padding: 10px 14px; border: 1px solid rgba(226,75,84,.12); border-radius: 12px; color: #c33e48; background: rgba(226,75,84,.05); font-size: 12px; }
	.dashboard-grid { display: grid; grid-template-columns: minmax(0, 1fr) 243px; gap: 32px; align-items: start; }.dashboard-main { min-width: 0; }
	.server-card, .side-card, .app-card, .recent-card { border: 1px solid rgba(18, 24, 34, .055); background: rgba(255,255,255,.88); box-shadow: 0 10px 30px rgba(15,23,42,.035), 0 2px 8px rgba(15,23,42,.025); backdrop-filter: blur(22px); }
	.server-card { box-sizing: border-box; height: 218px; display: grid; grid-template-columns: 132px minmax(250px,300px) 126px 160px; align-items: center; justify-content: center; column-gap: 28px; overflow: hidden; padding: 0 24px; border-radius: 21px; }
	.skeleton { background: linear-gradient(90deg,#fff,#f3f4f6,#fff); background-size: 200% 100%; animation: shimmer 1.5s infinite; }
	.empty-server { width: 100%; grid-template-columns: 1fr; place-content: center; gap: 4px; color: #1d2025; }.empty-server span { font-size: 25px; color: var(--dashboard-blue); }.empty-server strong { font-size: 17px; }.empty-server small { color: #888e97; }
	.server-device { width: 132px; height: 148px; display: grid; place-items: center; justify-self: center; }.server-device img { width: 132px; height: auto; max-width: none; filter: drop-shadow(0 14px 13px rgba(30,35,43,.16)); }
	.server-identity { min-width: 0; }.server-identity h2, .server-identity select { margin: 0; border: 0; color: #16181d; background: transparent; font-size: 21px; font-weight: 700; letter-spacing: -.025em; outline: none; }.server-identity select { max-width: 230px; padding: 0 22px 0 0; }.hardware-name { margin: 6px 0 0; color: #575c64; font-size: 13px; }.uptime { margin: 4px 0 15px; color: #8a8f98; font-size: 12px; }
	.status-chips { display: flex; gap: 7px; }.status-chips span { display: flex; align-items: center; gap: 8px; min-height: 31px; padding: 0 10px; border: 1px solid #e5e7e9; border-radius: 9px; background: rgba(255,255,255,.72); white-space: nowrap; font-size: 10px; }.status-chips b { font-weight: 600; }.status-chips em { color: #a2a6ad; font-style: normal; }.status-chips em.healthy { color: var(--dashboard-green); }.status-chips .blue-status { color: var(--dashboard-blue); }
	.cpu-block { display: grid; place-items: center; }.cpu-ring { width: 126px; height: 126px; display: grid; place-items: center; border-radius: 50%; background: conic-gradient(from 0deg, var(--dashboard-blue) 0 var(--progress), #eceef1 var(--progress) 360deg); box-shadow: inset 0 0 0 1px rgba(52,120,246,.03); }.cpu-ring::before { content: ''; position: absolute; width: 98px; height: 98px; border-radius: 50%; background: #fff; }.cpu-ring div { position: relative; z-index: 1; display: flex; flex-direction: column; align-items: center; }.cpu-ring strong { font-size: 25px; letter-spacing: -.04em; }.cpu-ring span { margin-top: 2px; color: #9297a0; font-size: 10px; }
	.section-heading { height: 52px; display: flex; align-items: center; justify-content: space-between; }.section-heading h2, .side-card h2 { margin: 0; font-size: 15px; font-weight: 680; letter-spacing: -.018em; }.section-heading button { border: 0; background: transparent; color: var(--dashboard-blue); font-size: 11px; font-weight: 560; }.section-heading button span { margin-left: 3px; font-size: 17px; vertical-align: -1px; }
	.section-heading-actions { display: flex; align-items: center; gap: 12px; }
	.installed-section { margin-top: 15px; }.apps-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }.app-card { position: relative; height: 111px; min-width: 0; padding: 0; border-radius: 17px; text-align: left; transition: transform .22s ease, box-shadow .22s ease; }.app-card:hover { transform: translateY(-2px); box-shadow: 0 15px 34px rgba(15,23,42,.07); }.app-card.menu-open { z-index: 20; transform: none; }.app-card-main { width: 100%; height: 100%; display: flex; align-items: center; gap: 12px; min-width: 0; padding: 17px 42px 17px 14px; border: 0; border-radius: inherit; background: transparent; text-align: left; }.app-menu { position: absolute; z-index: 2; top: 7px; right: 8px; width: 30px; height: 27px; display: grid; place-items: center; padding: 0 0 5px; border: 0; border-radius: 9px; color: #8f949c; background: transparent; font-size: 12px; letter-spacing: 1px; }.app-menu:hover,.app-menu[aria-expanded='true'] { color: #454a52; background: #f0f2f4; }.app-context-menu { position: absolute; z-index: 25; top: 37px; right: 8px; width: 144px; display: flex; flex-direction: column; gap: 2px; padding: 6px; border: 1px solid #e1e4e8; border-radius: 12px; background: rgba(255,255,255,.98); box-shadow: 0 16px 40px rgba(15,23,42,.16); backdrop-filter: blur(18px); }.app-context-menu button { width: 100%; min-height: 31px; padding: 0 9px; border: 0; border-radius: 8px; color: #4d525a; background: transparent; font-size: 10px; font-weight: 610; text-align: left; }.app-context-menu button:hover:not(:disabled) { color: #20242a; background: #f0f3f6; }.app-context-menu button:disabled { opacity: .45; }.app-copy { min-width: 0; display: flex; flex: 1; flex-direction: column; }.app-copy strong { overflow: hidden; color: #202329; font-size: 12px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }.app-copy small { margin-top: 3px; color: #969ba3; font-size: 10px; }.app-copy em { display: flex; align-items: center; gap: 5px; margin-top: 12px; color: #a0a5ad; font-size: 9px; font-style: normal; }.app-copy em i { width: 5px; height: 5px; border-radius: 50%; background: #b8bcc2; }.app-copy em.running { color: var(--dashboard-green); }.app-copy em.running i { background: var(--dashboard-green); box-shadow: 0 0 0 3px rgba(48,184,74,.08); }.app-placeholder { display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 2px; padding: 17px 14px; border-style: dashed; color: #747a83; }.app-placeholder span { color: var(--dashboard-blue); font-size: 20px; }.app-placeholder strong { font-size: 11px; }.app-placeholder small { color: #a0a5ac; font-size: 9px; }
	.app-context-menu button.danger { color: #c9404c; }.app-context-menu button.danger:hover { color: #b12f3b; background: #fff0f1; }
	.recent-card { margin-top: 18px; padding: 0 20px 16px; border-radius: 19px; }.recent-list { min-height: 90px; display: grid; grid-template-columns: repeat(5, minmax(0,1fr)); }.recent-list button { position: relative; min-width: 0; display: flex; align-items: center; flex-direction: column; padding: 8px 12px 4px; border: 0; background: transparent; }.recent-list button + button::before { content: ''; position: absolute; left: 0; top: 10px; bottom: 10px; width: 1px; background: #eceef0; }.file-icon { width: 37px; height: 34px; display: grid; place-items: center; color: #b5bbc3; }.file-icon svg { width: 30px; height: 30px; fill: #f4f5f6; stroke: #9298a1; stroke-width: 1.2; stroke-linejoin: round; }.file-icon.folder img { width: 37px; height: 37px; border-radius: 10px; object-fit: cover; }.recent-list strong { max-width: 100%; overflow: hidden; margin-top: 4px; color: #34383f; font-size: 10px; font-weight: 610; text-overflow: ellipsis; white-space: nowrap; }.recent-list small { margin-top: 3px; color: #a1a6ad; font-size: 9px; }
	.dashboard-side { display: flex; flex-direction: column; gap: 17px; }.side-card { border-radius: 19px; }.system-status { height: 297px; padding: 19px 18px 15px; }.side-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }.side-heading svg { width: 19px; height: 19px; fill: none; stroke: #33373d; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
	.metric-row { height: 57px; display: grid; grid-template-columns: 48px 34px 1fr; align-items: center; gap: 4px; border-bottom: 1px solid #eff0f2; }.metric-row > span { color: #666b74; font-size: 10px; }.metric-row > strong { color: #2f3339; font-size: 10px; font-weight: 650; }.spark { width: 100%; height: 34px; overflow: visible; fill: none; stroke-width: 1.7; }.spark .fill { stroke: none; opacity: .11; }.spark.cpu { stroke: var(--dashboard-blue); }.spark.cpu .fill { fill: var(--dashboard-blue); }.spark.ram { stroke: var(--dashboard-purple); }.spark.ram .fill { fill: var(--dashboard-purple); }.spark.storage { stroke: var(--dashboard-green); }.spark.storage .fill { fill: var(--dashboard-green); }
	.network-row { display: grid; grid-template-columns: 1fr auto; gap: 5px 10px; padding-top: 11px; font-size: 9px; }.network-row > span { grid-column: 1/-1; color: #41454c; font-size: 10px; font-weight: 620; }.network-row div { color: #7e838c; }.network-row b { color: #30343a; }
	.quick-actions { padding: 18px; }.quick-actions h2 { margin-bottom: 14px; }.quick-actions button { width: 100%; height: 45px; display: grid; grid-template-columns: 28px 1fr auto; align-items: center; gap: 8px; margin-top: 8px; padding: 0 11px; border: 1px solid #e8eaec; border-radius: 11px; background: rgba(255,255,255,.7); color: #363a41; text-align: left; }.quick-actions button:hover { border-color: #d9dce0; background: #fff; box-shadow: 0 6px 16px rgba(15,23,42,.04); }.quick-actions svg { width: 19px; height: 19px; fill: none; stroke: #3d4249; stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round; }.quick-actions span { font-size: 10px; font-weight: 580; }.quick-actions i { color: #a2a6ad; font-size: 17px; font-style: normal; }
	.widgets-section { margin-top: 31px; }.widgets-heading { min-height: 55px; display: flex; align-items: center; justify-content: space-between; gap: 16px; }.widgets-heading p { margin: 0 0 4px; color: #9a9fa7; font-size: 9px; font-weight: 720; letter-spacing: .11em; text-transform: uppercase; }.widgets-heading h2 { margin: 0; font-size: 17px; letter-spacing: -.025em; }.add-widget-button { height: 38px; display: flex; align-items: center; gap: 6px; padding: 0 13px; border: 1px solid #e1e4e7; border-radius: 11px; color: #3f444b; background: rgba(255,255,255,.82); box-shadow: 0 5px 16px rgba(15,23,42,.035); font-size: 10px; font-weight: 650; }.add-widget-button:hover { border-color: color-mix(in srgb,var(--dashboard-blue) 28%,#dfe2e6); color: var(--dashboard-blue); background: #fff; }.add-widget-button span { font-size: 16px; font-weight: 400; }.widgets-grid { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 14px; }.widgets-empty { width: 100%; min-height: 160px; display: flex; align-items: center; flex-direction: column; justify-content: center; border: 1px dashed #d9dde2; border-radius: 20px; color: #888e97; background: rgba(255,255,255,.46); }.widgets-empty > span { color: var(--dashboard-blue); font-size: 25px; }.widgets-empty strong { margin-top: 5px; color: #555a62; font-size: 12px; }.widgets-empty small { margin-top: 4px; font-size: 9px; }
	.custom-widget { --widget-accent: #3478f6; position: relative; min-width: 0; min-height: 246px; display: flex; flex-direction: column; overflow: hidden; padding: 17px 18px 15px; border: 1px solid rgba(18,24,34,.065); border-radius: 20px; background: linear-gradient(145deg,rgba(255,255,255,.96),rgba(252,253,254,.78)); box-shadow: 0 12px 34px rgba(15,23,42,.045),0 2px 7px rgba(15,23,42,.025); }.custom-widget::before { position: absolute; top: 0; right: 20px; left: 20px; height: 2px; content: ''; border-radius: 0 0 4px 4px; background: linear-gradient(90deg,transparent,var(--widget-accent),transparent); opacity: .65; }.custom-widget > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }.widget-title { min-width: 0; display: flex; align-items: center; gap: 10px; }.widget-title > span { width: 35px; height: 35px; display: grid; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--widget-accent); background: color-mix(in srgb,var(--widget-accent) 10%,white); }.widget-title svg { width: 19px; height: 19px; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }.widget-title > div { min-width: 0; display: flex; flex-direction: column; }.widget-title strong { overflow: hidden; color: #2e3238; font-size: 12px; font-weight: 680; text-overflow: ellipsis; white-space: nowrap; }.widget-title small { margin-top: 3px; color: #999ea6; font-size: 9px; }.widget-controls { display: flex; gap: 1px; }.widget-controls button { width: 25px; height: 25px; display: grid; place-items: center; padding: 0; border: 0; border-radius: 7px; color: #9a9fa7; background: transparent; font-size: 11px; }.widget-controls button:hover:not(:disabled) { color: #4e535b; background: #f0f2f4; }.widget-controls button:disabled { opacity: .25; }.widget-controls .remove-widget { font-size: 17px; }.widget-controls .remove-widget:hover:not(:disabled) { color: #d94a55; background: #fcebed; }
	.speed-result { flex: 1; display: grid; grid-template-columns: minmax(0,1.15fr) minmax(150px,.85fr); align-items: center; gap: 24px; padding: 18px 7px 12px; }.download-speed { min-width: 0; display: grid; grid-template-columns: auto auto; align-items: end; justify-content: start; column-gap: 6px; }.download-speed > span { grid-column: 1/-1; margin-bottom: 2px; color: #878d96; font-size: 9px; font-weight: 650; letter-spacing: .04em; text-transform: uppercase; }.download-speed > strong { color: #1e2228; font-size: clamp(34px,4vw,48px); line-height: .95; letter-spacing: -.055em; }.download-speed > small { padding-bottom: 4px; color: var(--widget-accent); font-size: 10px; font-weight: 680; }.speed-secondary { display: grid; gap: 10px; padding-left: 18px; border-left: 1px solid #e8eaed; }.speed-secondary div { display: flex; align-items: center; justify-content: space-between; gap: 12px; }.speed-secondary span { color: #8b9199; font-size: 9px; }.speed-secondary strong { max-width: 105px; overflow: hidden; color: #3c4148; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.speed-secondary strong small { color: #9a9fa7; font-size: 8px; font-weight: 550; }.speed-placeholder { flex: 1; display: flex; align-items: center; justify-content: center; gap: 16px; padding: 18px 5px 10px; }.speed-placeholder > div { display: flex; flex-direction: column; }.speed-placeholder strong { color: #50555d; font-size: 12px; }.speed-placeholder small { max-width: 230px; margin-top: 4px; color: #9a9fa7; font-size: 9px; line-height: 1.45; }.signal { width: 54px; height: 45px; display: flex; align-items: flex-end; justify-content: center; gap: 4px; padding: 8px; border-radius: 14px; background: color-mix(in srgb,var(--widget-accent) 8%,white); }.signal i { width: 5px; border-radius: 4px; background: var(--widget-accent); opacity: .35; }.signal i:nth-child(1) { height: 8px; }.signal i:nth-child(2) { height: 14px; opacity: .5; }.signal i:nth-child(3) { height: 21px; opacity: .7; }.signal i:nth-child(4) { height: 28px; opacity: .95; }.custom-widget > footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-top: 11px; border-top: 1px solid #eceef0; }.custom-widget > footer > span { overflow: hidden; color: #a0a5ad; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }.custom-widget > footer > .live-indicator { display: flex; flex: 0 0 auto; align-items: center; gap: 5px; color: #a0a5ad; font-weight: 750; letter-spacing: .07em; }.live-indicator i { width: 6px; height: 6px; border-radius: 50%; background: #b5b9bf; }.live-indicator.active { color: var(--dashboard-green); }.live-indicator.active i { background: var(--dashboard-green); box-shadow: 0 0 0 4px rgba(48,184,74,.1); animation: live-pulse 1.6s ease-out infinite; }
	.widget-builder { position: relative; width: min(470px,100%); overflow: hidden; border: 1px solid rgba(0,0,0,.07); border-radius: 24px; color: #292c31; background: rgba(255,255,255,.98); box-shadow: 0 34px 90px rgba(15,23,42,.18); }.widget-builder > header { display: flex; align-items: center; justify-content: space-between; padding: 21px 23px 17px; border-bottom: 1px solid #eceef0; }.widget-builder > header p { margin: 0 0 4px; color: #9a9fa7; font-size: 8px; font-weight: 720; letter-spacing: .12em; text-transform: uppercase; }.widget-builder > header h2 { margin: 0; font-size: 20px; letter-spacing: -.03em; }.widget-builder > header button { width: 31px; height: 31px; border: 0; border-radius: 9px; color: #858a92; background: transparent; font-size: 20px; }.widget-builder > header button:hover { background: #f0f2f4; }.widget-builder form { display: flex; flex-direction: column; padding: 19px 23px 22px; }.widget-builder form > label,.widget-builder .field-label { margin: 14px 0 7px; color: #565b63; font-size: 10px; font-weight: 650; }.widget-builder form > .widget-type-label { margin-top: 0; }.widget-type { min-height: 64px; display: grid; grid-template-columns: 39px 1fr 20px; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid color-mix(in srgb,var(--dashboard-blue) 25%,#dfe2e6); border-radius: 13px; background: #f6f9ff; }.widget-type > span { width: 37px; height: 37px; display: grid; place-items: center; border-radius: 11px; color: var(--dashboard-blue); background: #fff; box-shadow: 0 3px 10px rgba(20,55,100,.07); }.widget-type svg { width: 20px; height: 20px; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; }.widget-type > div { display: flex; flex-direction: column; }.widget-type strong { color: #353941; font-size: 11px; }.widget-type small { margin-top: 3px; color: #9499a2; font-size: 8px; }.widget-type > i { color: var(--dashboard-blue); font-size: 12px; font-style: normal; }.widget-builder input { height: 42px; padding: 0 12px; border: 1px solid #dfe2e6; border-radius: 11px; color: #30343a; background: #fafbfc; outline: none; }.widget-builder input:focus { border-color: color-mix(in srgb,var(--dashboard-blue) 45%,#dfe2e6); box-shadow: 0 0 0 3px rgba(52,120,246,.08); }.color-picker { display: flex; gap: 8px; }.color-picker button { width: 33px; height: 33px; display: grid; place-items: center; padding: 0; border: 1px solid transparent; border-radius: 10px; background: transparent; }.color-picker button span { width: 21px; height: 21px; border-radius: 50%; background: var(--swatch); box-shadow: inset 0 0 0 1px rgba(0,0,0,.06); }.color-picker button.active { border-color: var(--swatch); background: color-mix(in srgb,var(--swatch) 8%,white); }.widget-builder-actions { display: flex; justify-content: flex-end; gap: 7px; margin-top: 22px; }.widget-builder-actions button { padding: 9px 13px; border: 0; border-radius: 9px; color: #696e76; background: transparent; font-size: 10px; font-weight: 640; }
	.customization-builder { width: min(510px,100%); }.customization-preview { min-height: 78px; display: flex; align-items: center; gap: 14px; margin-bottom: 4px; padding: 11px 13px; border: 1px solid #e7e9ec; border-radius: 15px; background: #fafbfc; }.customization-preview > span { min-width: 0; display: flex; flex-direction: column; }.customization-preview strong { overflow: hidden; color: #353941; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.customization-preview small { margin-top: 4px; color: #969ba3; font-size: 9px; }.customization-builder label > small { color: #a1a6ae; font-weight: 500; }.customization-builder .icon-upload { position: relative; min-height: 40px; display: grid; place-items: center; margin-top: 11px; overflow: hidden; border: 1px dashed #d7dbe0; border-radius: 11px; color: #656b74; background: #fafbfc; cursor: pointer; }.customization-builder .icon-upload:hover { border-color: #bfc6cf; background: #f6f8fa; }.customization-builder .icon-upload input { position: absolute; width: 1px; height: 1px; opacity: 0; pointer-events: none; }.customization-builder .icon-upload span { font-size: 10px; font-weight: 650; }.customization-help { margin: 10px 1px 0; color: #9298a1; font-size: 9px; line-height: 1.5; }.customization-help code { padding: 1px 4px; border-radius: 4px; color: #666d77; background: #f0f2f4; }
	.modal-layer { position: fixed; z-index: 80; inset: 0; display: grid; place-items: center; padding: 20px; }.modal-backdrop { position: absolute; inset: 0; border: 0; background: rgba(24,27,32,.3); backdrop-filter: blur(10px); }.pair-modal { position: relative; width: min(440px, 100%); padding: 28px; border: 1px solid rgba(0,0,0,.07); border-radius: 26px; color: #202329; background: rgba(255,255,255,.96); box-shadow: 0 34px 90px rgba(15,23,42,.18); }.pair-icon { width: 46px; height: 46px; display: grid; place-items: center; margin-bottom: 18px; border-radius: 15px; color: #fff; background: var(--dashboard-blue); font-size: 23px; }.pair-icon.success { background: var(--dashboard-green); }.modal-kicker { margin: 0 0 5px !important; color: #999ea6 !important; font-size: 10px !important; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; }.pair-modal h2 { margin: 0; font-size: 23px; letter-spacing: -.03em; }.pair-modal > p { margin: 8px 0 20px; color: #777d86; font-size: 13px; line-height: 1.55; }.pair-modal form { display: flex; flex-direction: column; gap: 8px; }.pair-modal label { color: #575c64; font-size: 11px; font-weight: 620; }.pair-modal input { height: 44px; padding: 0 13px; border: 1px solid #dfe2e6; border-radius: 12px; color: #22252a; background: #fafbfc; outline: none; }.pair-modal form small { color: #8a9099; font-size: 10px; line-height: 1.45; }.pair-modal form div, .modal-footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 14px; }.pair-modal button { padding: 10px 15px; border: 0; border-radius: 11px; color: #626770; background: transparent; font-size: 11px; font-weight: 620; }.pair-modal .modal-primary { color: #fff; background: var(--dashboard-blue); }
	.pair-modal.command-modal { width: min(760px, 100%); }.pair-install-command { min-width: 0; display: grid; grid-template-columns: auto minmax(0,1fr) auto; align-items: center; gap: 11px; padding: 10px 10px 10px 14px; border: 1px solid #dce1e7; border-radius: 14px; color: #d8dee9; background: #171b22; box-shadow: inset 0 1px 0 rgba(255,255,255,.035); }.pair-install-command > span { color: #57d879; font: 700 13px ui-monospace,SFMono-Regular,monospace; }.pair-install-command code { min-width: 0; overflow-x: auto; padding: 7px 2px; color: #e6eaf0; font: 11px/1.45 ui-monospace,SFMono-Regular,monospace; scrollbar-width: thin; white-space: nowrap; user-select: all; }.pair-install-command button { flex: 0 0 auto; white-space: nowrap; }.pair-automation { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 13px; }.pair-automation span { padding: 6px 9px; border-radius: 8px; color: #4f5967; background: #f0f3f6; font-size: 9px; font-weight: 620; }.pair-connection { display: flex; align-items: center; gap: 11px; margin-top: 18px; padding: 12px 14px; border: 1px solid #e2e5e9; border-radius: 13px; background: #fafbfc; }.pair-connection > i { width: 9px; height: 9px; flex: 0 0 auto; border-radius: 50%; background: #a9afb7; box-shadow: 0 0 0 5px rgba(125,132,143,.09); animation: live-pulse 1.6s ease-out infinite; }.pair-connection > span { display: flex; flex-direction: column; }.pair-connection strong { color: #4f555e; font-size: 11px; }.pair-connection small { margin-top: 3px; color: #969ca5; font-size: 9px; }.pair-connection.connected { border-color: rgba(48,184,74,.17); background: rgba(48,184,74,.055); }.pair-connection.connected > i { background: var(--dashboard-green); box-shadow: 0 0 0 5px rgba(48,184,74,.1); animation: none; }.pair-connection.connected strong { color: #268941; }
	.relay-ready { display: flex; flex-direction: column; gap: 3px; margin-top: 6px; padding: 12px; border: 1px solid rgba(35,168,107,.2); border-radius: 12px; background: rgba(35,168,107,.06); }.relay-ready strong { color: #256f50; font-size: 11px; }.relay-ready small { color: #638071 !important; }
	.customization-builder { max-height: calc(100dvh - 40px); }.customization-builder form { overflow-y: auto; }

	/* Platform material system: keep the dashboard in the selected appearance. */
	.dashboard-shell { color: var(--fg); color-scheme: inherit; background: radial-gradient(circle at 50% -12%, color-mix(in srgb,var(--surface-solid) 68%,transparent),transparent 38%),linear-gradient(145deg,var(--bg-elevated),var(--bg) 58%,var(--bg-elevated)); }
	.dashboard-topbar { position: sticky; z-index: 30; top: 0; height: 82px; background: color-mix(in srgb,var(--chrome) 88%,transparent); box-shadow: inset 0 1px 0 color-mix(in srgb,white 16%,transparent),0 12px 24px -24px color-mix(in srgb,var(--fg) 24%,transparent); backdrop-filter: blur(28px) saturate(1.5); -webkit-backdrop-filter: blur(28px) saturate(1.5); }
	.wordmark { color: var(--fg); font-size: 27px; letter-spacing: -.045em; }
	.search-bar { border-color: var(--border); color: var(--fg-subtle); background: color-mix(in srgb,var(--surface-raised) 86%,transparent); box-shadow: inset 0 1px 0 color-mix(in srgb,white 20%,transparent),var(--shadow-sm); }
	.search-bar:hover { border-color: var(--border-strong); background: var(--surface-hover); }
	.search-bar svg,.round-action,.profile-button { color: var(--fg); }
	.search-bar svg { stroke: currentColor; }
	.search-bar kbd { border-color: var(--border); color: var(--fg-subtle); background: var(--track); }
	.round-action:hover { border-color: var(--border); background: var(--surface); }
	.profile-button { border-color: var(--border); background: var(--track); box-shadow: inset 0 1px 0 color-mix(in srgb,white 22%,transparent); }
	.profile-menu,.app-context-menu { border-color: var(--border); color: var(--fg); background: color-mix(in srgb,var(--surface-raised) 96%,transparent); box-shadow: var(--shadow-md); transform-origin: top right; animation: dashboard-popover 220ms var(--motion-settle) both; }
	.profile-copy { border-color: var(--border); }.profile-copy span { color: var(--fg-subtle); }
	.profile-menu button,.app-context-menu button { color: var(--fg-muted); }.profile-menu button:hover,.app-context-menu button:hover:not(:disabled) { color: var(--fg); background: var(--track); }
	.welcome-copy h1 { color: var(--fg); font-size: clamp(24px,2.2vw,29px); }.welcome-copy p { color: var(--fg-subtle); }
	.server-card,.side-card,.app-card,.recent-card { border-color: var(--border); background: linear-gradient(145deg,color-mix(in srgb,var(--surface-raised) 90%,transparent),var(--surface)); box-shadow: inset 0 1px 0 color-mix(in srgb,white 18%,transparent),var(--shadow-sm); backdrop-filter: blur(24px) saturate(1.35); -webkit-backdrop-filter: blur(24px) saturate(1.35); }
	.server-card { border-radius: 24px; }.empty-server { color: var(--fg); }
	.skeleton { background: linear-gradient(90deg,var(--surface-raised),var(--track),var(--surface-raised)); background-size: 200% 100%; }
	.server-identity h2,.server-identity select { color: var(--fg); }.hardware-name { color: var(--fg-muted); }.uptime { color: var(--fg-subtle); }
	.status-chips span { border-color: var(--border); background: color-mix(in srgb,var(--surface-raised) 72%,transparent); }.status-chips em { color: var(--fg-subtle); }
	.cpu-ring { background: conic-gradient(from 0deg,var(--dashboard-blue) 0 var(--progress),var(--track) var(--progress) 360deg); }.cpu-ring::before { background: var(--surface-solid); }.cpu-ring span { color: var(--fg-subtle); }
	.section-heading h2,.side-card h2,.widgets-heading h2 { color: var(--fg); }
	.app-card { border-radius: 19px; }.app-menu { color: var(--fg-subtle); }.app-menu:hover,.app-menu[aria-expanded='true'] { color: var(--fg); background: var(--track); }
	.app-copy strong,.recent-list strong { color: var(--fg); }.app-copy small,.app-copy em,.recent-list small { color: var(--fg-subtle); }
	.app-placeholder { color: var(--fg-muted); }.app-placeholder small { color: var(--fg-subtle); }
	.recent-list button + button::before,.metric-row { border-color: var(--border); }.file-icon svg { fill: var(--bg-elevated); stroke: var(--fg-subtle); }
	.side-heading svg,.quick-actions svg { stroke: var(--fg-muted); }.metric-row > span,.network-row div { color: var(--fg-muted); }.metric-row > strong,.network-row > span,.network-row b { color: var(--fg); }
	.quick-actions button { border-color: var(--border); color: var(--fg); background: color-mix(in srgb,var(--surface-raised) 72%,transparent); }.quick-actions button:hover { border-color: var(--border-strong); background: var(--surface-hover); box-shadow: var(--shadow-sm); }.quick-actions i { color: var(--fg-subtle); }
	.widgets-heading p { color: var(--fg-subtle); }.add-widget-button { border-color: var(--border); color: var(--fg-muted); background: var(--surface); box-shadow: var(--shadow-sm); }.add-widget-button:hover { border-color: color-mix(in srgb,var(--dashboard-blue) 35%,var(--border)); color: var(--dashboard-blue); background: var(--surface-hover); }
	.widgets-empty { border-color: var(--border-strong); color: var(--fg-subtle); background: var(--surface); }.widgets-empty strong { color: var(--fg-muted); }
	.custom-widget { border-color: var(--border); background: linear-gradient(145deg,color-mix(in srgb,var(--surface-raised) 94%,transparent),var(--surface)); box-shadow: inset 0 1px 0 color-mix(in srgb,white 18%,transparent),var(--shadow-sm); }.widget-title > span,.signal { background: color-mix(in srgb,var(--widget-accent) 11%,var(--surface-solid)); }.widget-title strong,.download-speed > strong { color: var(--fg); }.widget-title small,.download-speed > span,.speed-secondary span,.speed-secondary strong small,.speed-placeholder small,.custom-widget > footer > span { color: var(--fg-subtle); }.speed-secondary { border-color: var(--border); }.speed-secondary strong,.speed-placeholder strong { color: var(--fg-muted); }.custom-widget > footer { border-color: var(--border); }
	.widget-controls button { color: var(--fg-subtle); }.widget-controls button:hover:not(:disabled) { color: var(--fg); background: var(--track); }
	.widget-builder,.pair-modal { border-color: var(--border); color: var(--fg); background: color-mix(in srgb,var(--surface-raised) 97%,transparent); box-shadow: var(--shadow-xl); transform-origin: center 25%; animation: dashboard-modal 340ms var(--motion-settle) both; backdrop-filter: blur(36px) saturate(1.45); -webkit-backdrop-filter: blur(36px) saturate(1.45); }
	.widget-builder > header { border-color: var(--border); }.widget-builder > header p,.widget-title small { color: var(--fg-subtle); }.widget-builder > header button { color: var(--fg-subtle); }.widget-builder > header button:hover { background: var(--track); }
	.widget-builder form > label,.widget-builder .field-label,.pair-modal label { color: var(--fg-muted); }.widget-builder input,.pair-modal input { border-color: var(--border); color: var(--fg); background: var(--track); }.widget-builder input:focus,.pair-modal input:focus { border-color: color-mix(in srgb,var(--accent) 58%,var(--border)); box-shadow: 0 0 0 4px var(--accent-soft); }
	.widget-type,.customization-preview,.pair-connection { border-color: var(--border); background: color-mix(in srgb,var(--accent-soft) 44%,var(--surface)); }.widget-type > span { background: var(--surface-solid); }.widget-type strong,.customization-preview strong,.pair-modal h2 { color: var(--fg); }.widget-type small,.customization-preview small,.customization-help,.pair-modal > p,.pair-modal form small,.pair-connection small { color: var(--fg-subtle); }
	.customization-builder .icon-upload { border-color: var(--border-strong); color: var(--fg-muted); background: var(--track); }.customization-builder .icon-upload:hover { border-color: var(--border-strong); background: var(--surface-hover); }.customization-help code,.pair-automation span { color: var(--fg-muted); background: var(--track); }
	.widget-builder-actions button,.pair-modal button { color: var(--fg-muted); }.modal-backdrop { background: color-mix(in srgb,#07080b 38%,transparent); animation: dashboard-scrim 180ms ease-out both; backdrop-filter: blur(9px) saturate(.85); -webkit-backdrop-filter: blur(9px) saturate(.85); }
	.pair-connection { background: var(--track); }.pair-connection strong { color: var(--fg-muted); }
	.relay-ready { border-color: color-mix(in srgb,var(--success) 25%,transparent); background: color-mix(in srgb,var(--success) 8%,transparent); }.relay-ready strong { color: var(--success); }.relay-ready small { color: var(--fg-subtle) !important; }

	@keyframes dashboard-popover { from { opacity: 0; transform: translateY(-5px) scale(.96); filter: blur(4px); } to { opacity: 1; transform: translateY(0) scale(1); filter: blur(0); } }
	@keyframes dashboard-modal { from { opacity: 0; transform: translateY(10px) scale(.97); filter: blur(7px); } to { opacity: 1; transform: translateY(0) scale(1); filter: blur(0); } }
	@keyframes dashboard-scrim { from { opacity: 0; } to { opacity: 1; } }

	@media (hover: hover) and (pointer: fine) { .server-card:hover,.side-card:hover,.app-card:hover,.recent-card:hover,.custom-widget:hover { border-color: var(--border-strong); box-shadow: inset 0 1px 0 color-mix(in srgb,white 22%,transparent),var(--shadow-md); }.app-card:hover { transform: translateY(-2px); } }
	@media (hover: none) { .app-card:hover { transform: none; box-shadow: var(--shadow-sm); } }
	@media (prefers-reduced-motion: reduce) { .profile-menu,.app-context-menu,.widget-builder,.pair-modal,.modal-backdrop { animation: none; }.live-indicator.active i,.pair-connection > i { animation: none; } }
	@media (prefers-reduced-transparency: reduce) { .dashboard-topbar,.server-card,.side-card,.app-card,.recent-card,.custom-widget,.profile-menu,.app-context-menu,.widget-builder,.pair-modal { background: var(--surface-solid); backdrop-filter: none; -webkit-backdrop-filter: none; }.modal-backdrop { backdrop-filter: none; -webkit-backdrop-filter: none; } }
	@media (prefers-contrast: more) { .server-card,.side-card,.app-card,.recent-card,.custom-widget,.widget-builder,.pair-modal { border-color: var(--border-strong); background: var(--surface-solid); } }
	@keyframes shimmer { to { background-position: -200% 0; } }@keyframes live-pulse { 50% { box-shadow: 0 0 0 7px rgba(48,184,74,0); } }
	@media (max-width: 640px) { .pair-modal.command-modal { max-height: calc(100dvh - 28px); overflow-y: auto; padding: 22px 18px; }.pair-install-command { grid-template-columns: auto minmax(0,1fr); }.pair-install-command button { grid-column: 1/-1; width: 100%; }.pair-automation { gap: 5px; }.pair-automation span { flex: 1 1 calc(50% - 5px); text-align: center; } }
	@media (max-width: 1160px) { .dashboard-content { width: calc(100vw - 48px); margin: 0 24px; }.server-card { grid-template-columns: 118px minmax(220px,270px) 118px; column-gap: 18px; padding-inline: 18px; }.server-device,.server-device img { width: 118px; }.status-chips span:nth-child(3) { display: none; } }
	@media (max-width: 900px) { .dashboard-grid { grid-template-columns: 1fr; }.dashboard-side { display: grid; grid-template-columns: 1fr 1fr; }.system-status { height: auto; }.apps-grid,.widgets-grid { grid-template-columns: repeat(2,minmax(0,1fr)); }.server-card { grid-template-columns: 124px minmax(230px,330px) 126px; column-gap: 28px; padding-inline: 22px; }.server-device,.server-device img { width: 124px; }.search-bar { width: min(465px,50vw); }.recent-list { overflow-x: auto; grid-template-columns: repeat(5,140px); }.widget-controls button:nth-child(-n+2) { display: none; } }
	@media (max-width: 640px) { .dashboard-shell { padding-bottom: 105px; }.dashboard-topbar { height: 76px; padding: 15px 16px; }.wordmark { font-size: 24px; }.search-bar { width: 44px; padding: 0; justify-content: center; border-radius: 14px; }.search-bar span,.search-bar kbd { display:none; }.round-action { display:none; }.dashboard-content { width: calc(100vw - 28px); margin: 0 14px; }.welcome-copy { margin-top: 10px; }.welcome-copy h1 { font-size: 23px; }.server-card { height: auto; min-height: 244px; grid-template-columns: 96px minmax(0,1fr); column-gap: 14px; padding: 22px 18px; }.server-device { width: 96px; height: 108px; }.server-device img { width: 106px; }.cpu-block { grid-column: 1/-1; }.cpu-ring { width: 110px; height: 110px; }.cpu-ring::before { width: 86px; height: 86px; }.status-chips { flex-wrap: wrap; }.apps-grid,.widgets-grid { grid-template-columns: 1fr; }.app-card { height: 104px; }.app-card-main { padding: 13px 40px 13px 11px; }.app-placeholder { padding: 13px 11px; }.app-card :global(img),.app-card :global(div[style*='width']) { width: 38px !important; height: 38px !important; }.dashboard-side { grid-template-columns: 1fr; }.recent-card { padding-inline: 13px; }.recent-list { grid-template-columns: repeat(5,120px); }.quick-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }.quick-actions h2 { grid-column: 1/-1; }.quick-actions button { margin: 0; }.profile-menu { right: -2px; }.widgets-section { margin-top: 24px; }.custom-widget { min-height: 235px; }.speed-result { gap: 12px; }.download-speed > strong { font-size: 35px; }.speed-secondary { padding-left: 12px; }.widget-builder { border-radius: 20px; } }

	/* FaroOS spatial redesign: a visibly layered control centre, not a flat list of cards. */
	.dashboard-shell { background: transparent; }
	.dashboard-topbar { top: 12px; width: min(1480px,calc(100% - 24px)); height: 74px; margin: 12px auto 30px; padding: 0 22px; border: 1px solid var(--border-strong); border-radius: 24px; background: color-mix(in srgb,var(--chrome) 94%,transparent); box-shadow: inset 0 1px 0 color-mix(in srgb,white 34%,transparent),var(--shadow-md); backdrop-filter: blur(38px) saturate(1.7); -webkit-backdrop-filter: blur(38px) saturate(1.7); }
	.dashboard-topbar::after { position: absolute; right: 24px; bottom: -18px; left: 24px; height: 18px; content: ''; background: linear-gradient(to bottom,color-mix(in srgb,var(--fg) 8%,transparent),transparent 60%); mask-image: linear-gradient(to bottom,black,transparent); opacity: .17; pointer-events: none; }
	.wordmark { color: var(--fg); font-size: 26px; font-weight: 760; }
	.search-bar { height: 46px; border-color: var(--border); border-radius: 15px; background: color-mix(in srgb,var(--surface-raised) 80%,transparent); box-shadow: inset 0 1px 0 color-mix(in srgb,white 22%,transparent); }
	.dashboard-content { width: min(1320px,calc(100% - 48px)); }
	.welcome-copy { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; margin: 0 2px 26px; }
	.welcome-copy h1 { margin-top: 7px; font-size: clamp(30px,3vw,42px); font-weight: 450; letter-spacing: -.045em; }
	.welcome-copy > div > p:not(.welcome-kicker) { margin-top: 8px; color: var(--fg-muted); font-size: 15px; }
	.welcome-kicker { margin: 0; color: var(--accent); font-size: 10px; font-weight: 760; letter-spacing: .12em; text-transform: uppercase; }
	.welcome-actions { display: flex; align-items: center; gap: 10px; padding-bottom: 3px; }
	.fleet-online { min-height: 42px; display: flex; align-items: center; gap: 9px; padding: 0 14px; border: 1px solid var(--border); border-radius: 14px; color: var(--fg-muted); background: color-mix(in srgb,var(--surface-raised) 68%,transparent); font-size: 11px; font-weight: 620; backdrop-filter: blur(20px); }
	.fleet-online i { width: 7px; height: 7px; border-radius: 50%; background: var(--success); box-shadow: 0 0 0 5px color-mix(in srgb,var(--success) 11%,transparent),0 0 15px color-mix(in srgb,var(--success) 55%,transparent); }
	.welcome-actions button { min-height: 42px; display: flex; align-items: center; gap: 7px; padding: 0 16px; border: 0; border-radius: 14px; color: white; background: linear-gradient(180deg,color-mix(in srgb,var(--accent) 82%,white),var(--accent)); box-shadow: inset 0 1px 0 rgba(255,255,255,.28),0 10px 26px color-mix(in srgb,var(--accent) 25%,transparent); font-size: 11px; font-weight: 680; }
	.welcome-actions button span { font-size: 17px; font-weight: 400; }
	.overview-strip { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: 12px; margin-bottom: 22px; }
	.overview-tile { position: relative; min-width: 0; min-height: 104px; display: grid; grid-template-columns: 46px minmax(0,1fr) auto; align-items: center; gap: 13px; overflow: hidden; padding: 14px 15px; border: 1px solid var(--border-strong); border-radius: 22px; color: var(--fg); background: color-mix(in srgb,var(--surface-raised) 82%,transparent); box-shadow: inset 0 1px 0 color-mix(in srgb,white 34%,transparent),0 12px 32px color-mix(in srgb,#28558c 8%,transparent); text-align: left; backdrop-filter: blur(24px) saturate(1.45); -webkit-backdrop-filter: blur(24px) saturate(1.45); }
	.overview-tile::after { position: absolute; right: -24px; bottom: -34px; width: 96px; height: 96px; border-radius: 50%; content: ''; background: var(--tile-glow); filter: blur(22px); opacity: .3; pointer-events: none; }
	.overview-tile > i { position: relative; z-index: 1; color: var(--fg-subtle); font-size: 20px; font-style: normal; }
	.overview-icon { position: relative; z-index: 1; width: 46px; height: 46px; display: grid; place-items: center; border-radius: 15px; color: var(--tile-color); background: color-mix(in srgb,var(--tile-color) 12%,var(--surface-solid)); box-shadow: inset 0 1px 0 color-mix(in srgb,white 45%,transparent); }
	.overview-icon svg { width: 23px; height: 23px; fill: none; stroke: currentColor; stroke-width: 1.65; stroke-linecap: round; stroke-linejoin: round; }
	.overview-copy { position: relative; z-index: 1; min-width: 0; display: flex; flex-direction: column; }.overview-copy small { color: var(--fg-subtle); font-size: .625rem; font-weight: 680; letter-spacing: .035em; text-transform: uppercase; }.overview-copy strong { overflow: hidden; margin-top: 2px; color: var(--fg); font-size: 1.25rem; line-height: 1.15; letter-spacing: -.035em; text-overflow: ellipsis; white-space: nowrap; }.overview-copy strong em { color: var(--fg-subtle); font-size: .75rem; font-style: normal; font-weight: 580; }.overview-copy > span { overflow: hidden; margin-top: 4px; color: var(--fg-muted); font-size: .625rem; text-overflow: ellipsis; white-space: nowrap; }
	.servers-tile { --tile-color: var(--accent); --tile-glow: color-mix(in srgb,var(--accent) 44%,transparent); }.cpu-tile { --tile-color: #af52de; --tile-glow: rgba(175,82,222,.38); }.memory-tile { --tile-color: #ff9f0a; --tile-glow: rgba(255,159,10,.4); }.storage-tile { --tile-color: var(--success); --tile-glow: color-mix(in srgb,var(--success) 42%,transparent); }
	.dashboard-grid { grid-template-columns: minmax(0,1fr) 286px; gap: 22px; }
	.server-card,.side-card,.app-card,.recent-card,.custom-widget { border-color: var(--border-strong); background: linear-gradient(145deg,color-mix(in srgb,var(--surface-raised) 92%,transparent),color-mix(in srgb,var(--surface) 90%,transparent)); box-shadow: inset 0 1px 0 color-mix(in srgb,white 32%,transparent),var(--shadow-sm); }
	.server-card { position: relative; height: 252px; grid-template-columns: 160px minmax(280px,1fr) 146px; column-gap: 44px; padding: 0 40px; border-radius: 30px; }
	.server-card,.side-card,.recent-card,.custom-widget { backdrop-filter: blur(32px) saturate(1.55); -webkit-backdrop-filter: blur(32px) saturate(1.55); }
	.app-card { backdrop-filter: blur(18px) saturate(1.35); -webkit-backdrop-filter: blur(18px) saturate(1.35); }
	.server-card::after { position: absolute; top: -100px; right: -80px; width: 310px; height: 310px; border-radius: 50%; content: ''; background: color-mix(in srgb,var(--accent) 12%,transparent); filter: blur(30px); pointer-events: none; }
	.server-card > * { position: relative; z-index: 1; }
	.server-device,.server-device img { width: 150px; }
	.server-identity h2,.server-identity select { font-size: 25px; font-weight: 730; }
	.hardware-name { margin-top: 8px; font-size: 14px; }.uptime { margin-top: 5px; margin-bottom: 18px; font-size: 12px; }
	.status-chips span { min-height: 34px; padding-inline: 11px; border-radius: 11px; }
	.cpu-ring { width: 138px; height: 138px; box-shadow: 0 16px 36px color-mix(in srgb,var(--accent) 13%,transparent); }.cpu-ring::before { width: 106px; height: 106px; }.cpu-ring strong { font-size: 28px; }
	.section-heading { height: 64px; padding-inline: 3px; }.section-heading h2,.side-card h2 { font-size: 17px; font-weight: 720; }
	.installed-section { margin-top: 18px; }.apps-grid { grid-template-columns: repeat(5,minmax(0,1fr)); gap: 13px; }.app-card { height: 144px; border-radius: 24px; }.app-card-main { align-items: center; justify-content: center; flex-direction: column; gap: 9px; padding: 16px 24px; text-align: center; }.app-card-main :global(img),.app-card-main :global(div[style*='width']) { width: 54px !important; height: 54px !important; border-radius: 15px !important; box-shadow: 0 10px 22px rgba(16,34,60,.13); }.app-copy { align-items: center; }.app-copy strong { max-width: 100%; font-size: .8125rem; letter-spacing: -.01em; }.app-copy small { font-size: .6875rem; letter-spacing: .01em; }.app-copy em { margin-top: 6px; font-size: .625rem; letter-spacing: .015em; }.app-placeholder { min-height: 144px; border-width: 1.5px; background: color-mix(in srgb,var(--surface) 44%,transparent); }
	.app-card[draggable='true'] { cursor: grab; }.app-card[draggable='true']:active { cursor: grabbing; }.app-card.dragging { z-index: 30; opacity: .5; scale: .96; }.app-card.drop-target { border-color: color-mix(in srgb,var(--accent) 70%,var(--border)); box-shadow: inset 0 0 0 2px color-mix(in srgb,var(--accent) 24%,transparent),0 16px 42px color-mix(in srgb,var(--accent) 18%,transparent); translate: 0 -3px; }
	.recent-card { margin-top: 22px; padding: 0 22px 20px; border-radius: 25px; }.recent-list { min-height: 110px; }
	.dashboard-side { gap: 18px; }.side-card { border-radius: 25px; }.system-status { height: 330px; padding: 23px 21px 17px; }.metric-row { height: 64px; grid-template-columns: 54px 36px 1fr; }.metric-row > span,.metric-row > strong { font-size: .6875rem; letter-spacing: .01em; }.quick-actions { padding: 21px; }.quick-actions button { height: 50px; margin-top: 9px; border-radius: 14px; }.quick-actions button span { font-size: .6875rem; letter-spacing: .005em; }
	.widgets-section { margin-top: 38px; }.widgets-heading { min-height: 68px; }.widgets-heading h2 { font-size: 22px; }.custom-widget { border-radius: 26px; }
	.html-widget { min-height: 330px; }.html-widget-frame { flex: 1; min-height: 230px; overflow: hidden; margin: 14px 0 12px; border: 1px solid var(--border); border-radius: 17px; background: #fff; box-shadow: inset 0 1px 4px rgba(15,23,42,.08); }.html-widget-frame iframe { width: 100%; height: 100%; display: block; border: 0; background: #fff; }.html-indicator { display: flex; flex: 0 0 auto; align-items: center; gap: 5px; color: var(--widget-accent); font-size: 8px; font-weight: 750; letter-spacing: .07em; }.html-indicator i { width: 6px; height: 6px; border-radius: 50%; background: var(--widget-accent); box-shadow: 0 0 0 4px color-mix(in srgb,var(--widget-accent) 10%,transparent); }
	.widget-builder.html-editor { width: min(720px,100%); }.widget-type-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 9px; }.widget-type { width: 100%; color: var(--fg); text-align: left; cursor: pointer; }.widget-type:not(.selected) { border-color: var(--border); background: var(--track); }.widget-type.selected { border-color: color-mix(in srgb,var(--accent) 35%,var(--border)); background: color-mix(in srgb,var(--accent-soft) 50%,var(--surface)); }.widget-builder textarea { width: 100%; min-height: 220px; resize: vertical; padding: 13px; border: 1px solid var(--border); border-radius: 13px; color: var(--fg); background: color-mix(in srgb,var(--bg) 74%,var(--surface-solid)); outline: none; font: .72rem/1.55 ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace; tab-size: 2; }.widget-builder textarea:focus { border-color: color-mix(in srgb,var(--accent) 58%,var(--border)); box-shadow: 0 0 0 4px var(--accent-soft); }.html-widget-help { margin: 7px 1px 0; color: var(--fg-subtle); font-size: .625rem; line-height: 1.5; }

	@media (max-width: 1160px) {
		.dashboard-content { width: calc(100% - 40px); margin-inline: 20px; }
		.overview-strip { grid-template-columns: repeat(2,minmax(0,1fr)); }
		.server-card { grid-template-columns: 132px minmax(230px,1fr) 126px; column-gap: 22px; padding-inline: 24px; }
		.server-device,.server-device img { width: 132px; }
	}
	@media (max-width: 900px) {
		.dashboard-grid { grid-template-columns: 1fr; }
		.dashboard-side { grid-template-columns: 1fr 1fr; }
		.server-card { grid-template-columns: 140px minmax(230px,1fr) 138px; }
		.apps-grid { grid-template-columns: repeat(3,minmax(0,1fr)); }
	}
	@media (max-width: 640px) {
		.dashboard-topbar { top: 8px; width: calc(100% - 16px); height: 66px; margin: 8px auto 26px; padding: 0 12px 0 15px; border-radius: 20px; }
		.wordmark { font-size: 23px; }.dashboard-content { width: calc(100% - 24px); margin-inline: 12px; }
		.welcome-copy { align-items: flex-start; flex-direction: column; gap: 16px; margin: 0 3px 22px; }.welcome-copy h1 { font-size: 29px; }.welcome-copy > div > p:not(.welcome-kicker) { font-size: 13px; line-height: 1.45; }
		.welcome-actions { width: 100%; }.fleet-online { flex: 1; }.welcome-actions button { flex: 0 0 auto; }
		.server-card { height: 340px; min-height: 340px; grid-template-columns: 108px minmax(0,1fr); column-gap: 14px; padding: 24px 18px; border-radius: 26px; }
		.server-device,.server-device img { width: 108px; }.server-identity h2,.server-identity select { font-size: 21px; }.status-chips span:nth-child(3) { display: none; }
		.cpu-block { margin-top: 5px; }.cpu-ring { width: 116px; height: 116px; }.cpu-ring::before { width: 90px; height: 90px; }
		.overview-strip { gap: 9px; margin-bottom: 18px; }.overview-tile { min-height: 94px; grid-template-columns: 38px minmax(0,1fr); gap: 10px; padding: 12px; border-radius: 20px; }.overview-tile > i { display: none; }.overview-icon { width: 38px; height: 38px; border-radius: 12px; }.overview-icon svg { width: 19px; height: 19px; }.overview-copy strong { font-size: 1rem; }.overview-copy > span { font-size: .5625rem; }
		.section-heading { height: 58px; }.apps-grid,.widgets-grid { grid-template-columns: 1fr; }.app-card { height: 116px; }.app-card-main { align-items: center; justify-content: flex-start; flex-direction: row; gap: 13px; padding: 13px 40px 13px 13px; text-align: left; }.app-card-main :global(img),.app-card-main :global(div[style*='width']) { width: 44px !important; height: 44px !important; border-radius: 12px !important; }.app-copy { align-items: flex-start; }.app-copy em { margin-top: 8px; }.app-placeholder { min-height: 116px; }
		.dashboard-side { grid-template-columns: 1fr; }.system-status { height: auto; }.quick-actions { grid-template-columns: 1fr 1fr; }
		.widget-type-grid { grid-template-columns: 1fr; }.widget-builder.html-editor { max-height: calc(100dvh - 28px); overflow-y: auto; }.widget-builder textarea { min-height: 180px; }.html-widget { min-height: 300px; }
	}
</style>
