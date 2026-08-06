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
		...init
	});
	if (!res.ok) {
		throw new Error(`${init?.method ?? 'GET'} ${path} failed: ${res.status}`);
	}
	return res.json() as Promise<T>;
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
