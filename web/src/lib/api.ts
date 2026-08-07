// Thin client for the FaroOS server API. Same-origin in production (the Go
// server serves this build directly); during `vite dev` set PUBLIC_API_BASE
// via .env to point at a running `go run ./cmd/server` instance.
const API_BASE = import.meta.env.VITE_API_BASE ?? '';

export interface Disk {
	mountPoint: string;
	filesystem: string;
	totalBytes: number;
	usedBytes: number;
}

export interface Stats {
	cpuPercent: number;
	memUsedBytes: number;
	memTotalBytes: number;
	diskUsedBytes: number;
	diskTotalBytes: number;
	disks?: Disk[];
	uptimeSeconds: number;
	timestamp: string;
}

export interface Node {
	id: string;
	name: string;
	connected: boolean;
	pairedAt: string;
	lastSeen: string;
	stats: Stats;
}

export interface PairingResult {
	id: string;
	name: string;
	token: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`${API_BASE}${path}`, {
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		...init
	});
	if (res.status === 401 && !path.startsWith('/api/auth') && typeof window !== 'undefined') {
		const { goto } = await import('$app/navigation');
		await goto('/login');
	}
	if (!res.ok) {
		throw new Error(`${init?.method ?? 'GET'} ${path} failed: ${res.status}`);
	}
	return res.json() as Promise<T>;
}

export interface AuthStatus {
	needsSetup: boolean;
	authenticated: boolean;
}

export function authStatus(): Promise<AuthStatus> {
	return request<AuthStatus>('/api/auth/status');
}

export function setupAdmin(username: string, password: string): Promise<{ ok: boolean }> {
	return request('/api/auth/setup', { method: 'POST', body: JSON.stringify({ username, password }) });
}

export function login(username: string, password: string): Promise<{ ok: boolean }> {
	return request('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
}

export function logout(): Promise<{ ok: boolean }> {
	return request('/api/auth/logout', { method: 'POST' });
}

export function listNodes(): Promise<Node[]> {
	return request<Node[]>('/api/nodes');
}

export function createPairing(name: string): Promise<PairingResult> {
	return request<PairingResult>('/api/nodes', {
		method: 'POST',
		body: JSON.stringify({ name })
	});
}

export interface ContainerPort {
	privatePort: number;
	publicPort?: number;
	type: string;
}

export interface Container {
	id: string;
	names: string[];
	image: string;
	state: string;
	status: string;
	ports: ContainerPort[];
	labels: Record<string, string>;
	created: number;
}

export function listContainers(nodeId: string): Promise<Container[]> {
	return request<Container[]>(`/api/nodes/${nodeId}/containers`);
}

export type ContainerAction = 'start' | 'stop' | 'restart';

export function containerAction(nodeId: string, containerId: string, action: ContainerAction): Promise<{ ok: boolean }> {
	return request(`/api/nodes/${nodeId}/containers/${containerId}/${action}`, { method: 'POST' });
}

export function containerLogs(nodeId: string, containerId: string, tail = 200): Promise<{ logs: string }> {
	return request(`/api/nodes/${nodeId}/containers/${containerId}/logs?tail=${tail}`);
}

export interface FileEntry {
	name: string;
	isDir: boolean;
	size: number;
	modTime: string;
}

export function listFiles(nodeId: string, path: string): Promise<FileEntry[]> {
	return request<FileEntry[]>(`/api/nodes/${nodeId}/files?path=${encodeURIComponent(path)}`);
}

export function fileDownloadUrl(nodeId: string, path: string): string {
	return `${API_BASE}/api/nodes/${nodeId}/files/download?path=${encodeURIComponent(path)}`;
}

export async function uploadFile(nodeId: string, path: string, file: File): Promise<void> {
	const res = await fetch(`${API_BASE}/api/nodes/${nodeId}/files/upload?path=${encodeURIComponent(path)}`, {
		method: 'POST',
		credentials: 'include',
		body: file
	});
	if (!res.ok) {
		throw new Error(`upload failed: ${res.status}`);
	}
}

export function deleteFile(nodeId: string, path: string): Promise<{ ok: boolean }> {
	return request(`/api/nodes/${nodeId}/files?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
}

export interface AppPort {
	container: number;
	host: number;
	protocol: string;
}

export interface AppVolume {
	name: string;
	container: string;
}

export interface AppEnvVar {
	key: string;
	default: string;
	description?: string;
}

export interface CatalogApp {
	id: string;
	name: string;
	description: string;
	icon?: string;
	image: string;
	category?: string;
	source: 'faroos' | 'unraid-ca';
	ports: AppPort[];
	volumes: AppVolume[];
	env?: AppEnvVar[];
}

export function listApps(): Promise<CatalogApp[]> {
	return request<CatalogApp[]>('/api/apps');
}

export function listAppCategories(): Promise<string[]> {
	return request<string[]>('/api/apps/categories');
}

export function refreshAppCatalog(): Promise<{ ok: boolean; count: number }> {
	return request('/api/apps/refresh', { method: 'POST' });
}

export interface DeployOverrides {
	ports: AppPort[];
	env: AppEnvVar[];
}

export function deployApp(nodeId: string, appId: string, overrides: DeployOverrides): Promise<{ ok: boolean }> {
	return request(`/api/nodes/${nodeId}/apps/${appId}/deploy`, {
		method: 'POST',
		body: JSON.stringify(overrides)
	});
}

export function removeApp(nodeId: string, appId: string): Promise<{ ok: boolean }> {
	return request(`/api/nodes/${nodeId}/apps/${appId}/remove`, { method: 'POST' });
}

export interface PortStatus {
	port: number;
	inUse: boolean;
	ownApp: boolean;
	containerId?: string;
	containerName?: string;
}

export function inspectPort(nodeId: string, port: number): Promise<PortStatus> {
	return request(`/api/nodes/${nodeId}/ports/${port}`);
}

export function terminalWsUrl(nodeId: string, cols: number, rows: number): string {
	const origin =
		API_BASE || (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8090');
	const wsBase = origin.replace(/^http/, 'ws');
	return `${wsBase}/api/nodes/${nodeId}/terminal?cols=${cols}&rows=${rows}`;
}
