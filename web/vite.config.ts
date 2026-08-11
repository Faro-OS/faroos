import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) => filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			version: {
				// Keep already-open FaroOS tabs in sync with the panel binary.
				// When an update lands, SvelteKit notices the new build and the
				// root layout reloads the interface without user intervention.
				pollInterval: 15_000
			},
			adapter: adapter({
				// Output straight into the Go module so cmd/server can
				// go:embed it (see internal/webui) — the binary ships with
				// its own UI rather than needing web/build shipped
				// separately alongside it at the right relative path.
				pages: '../internal/webui/dist',
				assets: '../internal/webui/dist',
				fallback: 'index.html',
				precompress: false,
				strict: true
			})
		})
	]
});
