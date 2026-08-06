// Thin client for the FaroOS server API. Same-origin in production (the Go
// server serves this build directly); during `vite dev` set PUBLIC_API_BASE
// via .env to point at a running `go run ./cmd/server` instance.
const API_BASE = import.meta.env.VITE_API_BASE ?? '';

export interface Stats {
	cpuPercent: number;
	memUsedBytes: number;
	memTotalBytes: number;
	diskUsedBytes: number;
	diskTotalBytes: number;
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

export function terminalWsUrl(nodeId: string, cols: number, rows: number): string {
	const origin =
		API_BASE || (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8090');
	const wsBase = origin.replace(/^http/, 'ws');
	return `${wsBase}/api/nodes/${nodeId}/terminal?cols=${cols}&rows=${rows}`;
}
