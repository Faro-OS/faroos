type Theme = 'light' | 'dark';

function systemPrefersDark(): boolean {
	return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function loadInitial(): Theme {
	if (typeof window === 'undefined') return 'dark';
	const stored = window.localStorage.getItem('faroos-theme');
	if (stored === 'light' || stored === 'dark') return stored;
	return systemPrefersDark() ? 'dark' : 'light';
}

let theme = $state<Theme>(loadInitial());

export function getTheme(): Theme {
	return theme;
}

export function toggleTheme(): void {
	setTheme(theme === 'dark' ? 'light' : 'dark');
}

export function setTheme(next: Theme): void {
	theme = next;
	if (typeof document !== 'undefined') {
		document.documentElement.dataset.theme = next;
		window.localStorage.setItem('faroos-theme', next);
	}
}
