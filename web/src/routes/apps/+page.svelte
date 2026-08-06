<script lang="ts">
	import TopBar from '$lib/components/TopBar.svelte';

	const apps = [
		{ name: 'Nextcloud', description: 'A private cloud for files, calendars, and collaboration.' },
		{ name: 'Immich', description: 'Self-hosted photo and video backup for your library.' },
		{ name: 'Vaultwarden', description: 'A lightweight password manager compatible with Bitwarden.' },
		{ name: 'Jellyfin', description: 'Stream your movies, shows, music, and photos from home.' },
		{ name: 'Pi-hole', description: 'Network-wide ad and tracker blocking for every device.' },
		{ name: 'Uptime Kuma', description: 'A friendly monitoring dashboard for services and websites.' }
	];
</script>

<TopBar title="App Store" />

<main class="flex-1 p-6">
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
		{#each apps as app (app.name)}
			<article class="flex min-h-48 flex-col rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
				<div class="mb-4 grid h-11 w-11 place-items-center rounded-xl bg-[var(--track)] text-lg font-semibold text-[var(--accent)]">
					{app.name.charAt(0)}
				</div>
				<h2 class="font-semibold text-[var(--fg)]">{app.name}</h2>
				<p class="mt-1 flex-1 text-sm leading-6 text-[var(--fg-muted)]">{app.description}</p>
				<button
					type="button"
					disabled
					class="mt-5 w-full cursor-not-allowed rounded-xl bg-[var(--track)] px-4 py-2 text-sm font-semibold text-[var(--fg-subtle)]"
				>
					Coming soon
				</button>
			</article>
		{/each}
	</div>
</main>
