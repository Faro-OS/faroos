// FaroOS is a single-page control panel — sections switch instantly in
// place (like CasaOS/a real dashboard), not via full page navigation to
// separate routes. This is the shared "which section is showing" state,
// used by both Sidebar (sets it) and the root +page.svelte (reads it to
// decide which section component to mount).
export type SectionId =
	| 'dashboard'
	| 'servers'
	| 'containers'
	| 'storage'
	| 'apps'
	| 'terminal'
	| 'files'
	| 'settings';

let active = $state<SectionId>('dashboard');

export function getSection(): SectionId {
	return active;
}

export function setSection(id: SectionId): void {
	active = id;
}
