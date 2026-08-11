export function formatBytes(bytes: number): string {
	if (!bytes) return '0 B';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let value = bytes;
	let unitIndex = 0;
	while (value >= 1024 && unitIndex < units.length - 1) {
		value /= 1024;
		unitIndex++;
	}
	return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

export function formatStorage(bytes: number): string {
	if (!bytes) return '0 GB';
	const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
	let value = bytes;
	let unitIndex = 0;
	while (value >= 1000 && unitIndex < units.length - 1) {
		value /= 1000;
		unitIndex++;
	}
	return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

export function formatMemory(bytes: number): string {
	if (!bytes) return '0 GB';
	if (bytes >= 1_000_000_000) return `${Math.round(bytes / 1_000_000_000)} GB`;
	if (bytes >= 1_000_000) return `${Math.round(bytes / 1_000_000)} MB`;
	return `${Math.round(bytes / 1000)} KB`;
}

export function totalStorageBytes(stats: { diskTotalBytes: number; disks?: { totalBytes: number }[] }): number {
	const detected = stats.disks?.reduce((sum, disk) => sum + Math.max(0, disk.totalBytes), 0) ?? 0;
	return detected || stats.diskTotalBytes;
}

export function formatUptime(seconds: number): string {
	if (!seconds) return '—';
	const days = Math.floor(seconds / 86400);
	const hours = Math.floor((seconds % 86400) / 3600);
	if (days > 0) return `${days}d ${hours}h`;
	const minutes = Math.floor((seconds % 3600) / 60);
	if (hours > 0) return `${hours}h ${minutes}m`;
	return `${minutes}m`;
}

export function formatRelativeTime(iso: string): string {
	if (!iso) return '—';
	const diffMs = Date.now() - new Date(iso).getTime();
	const diffSec = Math.round(diffMs / 1000);
	if (diffSec < 5) return 'just now';
	if (diffSec < 60) return `${diffSec}s ago`;
	const diffMin = Math.round(diffSec / 60);
	if (diffMin < 60) return `${diffMin}m ago`;
	const diffHour = Math.round(diffMin / 60);
	return `${diffHour}h ago`;
}
