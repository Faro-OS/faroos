// Mobile drawer state for the sidebar — shared between TopBar (the
// hamburger toggle) and Sidebar (the drawer itself), which don't have a
// direct parent/child relationship (TopBar is rendered per-page, Sidebar
// lives in the root layout).
let open = $state(false);

export function isSidebarOpen(): boolean {
	return open;
}

export function toggleSidebar(): void {
	open = !open;
}

export function closeSidebar(): void {
	open = false;
}
