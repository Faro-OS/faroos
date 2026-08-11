import type { Container } from '$lib/api';

// Published does not automatically mean browser-accessible. Databases,
// message brokers and mail services often expose ports too, but presenting
// those as clickable dashboard apps only leads to dead browser tabs.
const NON_WEB_PORTS = new Set([
	21, 22, 23, 25, 53, 110, 143, 389, 465, 587, 636, 993, 995, 1433, 1521, 1883, 2375, 2376,
	3306, 5432, 5672, 6379, 9092, 27017
]);

const PREFERRED_WEB_PORTS = [443, 8443, 9443, 80, 8080, 8000, 3000, 5000, 9000];

const EXTERNAL_APP_ICONS = {
	immich: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/immich.svg',
	minio: 'https://cdn.simpleicons.org/minio/C72E49',
	wordpress: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/wordpress.svg',
	umbrel: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/umbrel.svg',
	odoo: 'https://cdn.simpleicons.org/odoo/714B67',
	cloudflare: 'https://cdn.simpleicons.org/cloudflare/F38020',
	snappymail: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/snappymail.svg',
	dockerMailserver: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/docker-mailserver.svg',
	mail: 'https://cdn.jsdelivr.net/npm/@mdi/svg@7.4.47/svg/email.svg'
} as const;

export function containerWebPort(container: Container): number | undefined {
	const candidates = container.ports.filter(
		(port) =>
			port.publicPort &&
			!NON_WEB_PORTS.has(port.privatePort) &&
			!NON_WEB_PORTS.has(port.publicPort)
	);
	if (candidates.length === 0) return undefined;
	candidates.sort((a, b) => {
		const aRank = PREFERRED_WEB_PORTS.indexOf(a.privatePort);
		const bRank = PREFERRED_WEB_PORTS.indexOf(b.privatePort);
		if (aRank === -1 && bRank === -1) return 0;
		if (aRank === -1) return 1;
		if (bRank === -1) return -1;
		return aRank - bRank;
	});
	return candidates[0]?.publicPort;
}

export function containerAppName(container: Container): string {
	return (
		container.labels?.['org.opencontainers.image.title'] ||
		container.labels?.['com.docker.compose.service'] ||
		container.names[0]?.replace(/^\//, '') ||
		container.image.split('/').pop()?.split(':')[0] ||
		'Container app'
	);
}

export function containerAppIcon(container: Container): string | undefined {
	const fingerprint = [
		...container.names,
		container.image,
		container.labels?.['org.opencontainers.image.title'],
		container.labels?.['com.docker.compose.service']
	]
		.filter(Boolean)
		.join(' ')
		.toLowerCase();

	if (fingerprint.includes('immich')) return EXTERNAL_APP_ICONS.immich;
	if (fingerprint.includes('minio')) return EXTERNAL_APP_ICONS.minio;
	if (fingerprint.includes('wordpress')) return EXTERNAL_APP_ICONS.wordpress;
	if (fingerprint.includes('umbrel')) return EXTERNAL_APP_ICONS.umbrel;
	if (fingerprint.includes('odoo')) return EXTERNAL_APP_ICONS.odoo;
	if (fingerprint.includes('cloudflare')) return EXTERNAL_APP_ICONS.cloudflare;
	if (fingerprint.includes('snappymail')) return EXTERNAL_APP_ICONS.snappymail;
	if (fingerprint.includes('docker-mailserver')) return EXTERNAL_APP_ICONS.dockerMailserver;
	if (/(^|[\s/_-])(webmail|mailserver|mail-api|mail)([\s/:_-]|$)/.test(fingerprint)) return EXTERNAL_APP_ICONS.mail;

	return container.labels?.['net.unraid.docker.icon'] || container.labels?.['org.opencontainers.image.icon'];
}
