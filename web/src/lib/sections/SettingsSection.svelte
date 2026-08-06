<script lang="ts">
	import { goto } from '$app/navigation';
	import TopBar from '$lib/components/TopBar.svelte';
	import { getTheme, toggleTheme } from '$lib/theme.svelte';
	import { logout } from '$lib/api';

	let loggingOut = $state(false);

	async function handleLogout() {
		loggingOut = true;
		try {
			await logout();
		} finally {
			await goto('/login');
		}
	}
</script>

<TopBar title="Settings" />

<main class="flex-1 p-6">
	<div class="mx-auto flex max-w-3xl flex-col gap-4">
		<section class="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
			<div class="flex items-center justify-between gap-6">
				<div>
					<h2 class="font-semibold text-[var(--fg)]">Appearance</h2>
					<p class="mt-1 text-sm text-[var(--fg-muted)]">
						Use {getTheme() === 'dark' ? 'dark' : 'light'} mode across FaroOS.
					</p>
				</div>
				<button
					type="button"
					role="switch"
					aria-label="Use dark mode"
					aria-checked={getTheme() === 'dark'}
					onclick={toggleTheme}
					class="relative h-7 w-12 shrink-0 rounded-full transition-colors {getTheme() === 'dark'
						? 'bg-[var(--accent)]'
						: 'bg-[var(--track)]'}"
				>
					<span
						class="absolute top-1 h-5 w-5 rounded-full bg-[var(--accent-fg)] transition-transform {getTheme() ===
						'dark'
							? 'translate-x-6'
							: 'translate-x-1'}"
					></span>
				</button>
			</div>
		</section>

		<section class="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
			<h2 class="font-semibold text-[var(--fg)]">About</h2>
			<div class="mt-4 flex items-center justify-between gap-6">
				<div>
					<p class="font-medium text-[var(--fg)]">FaroOS</p>
					<p class="mt-1 text-sm text-[var(--fg-muted)]">Open-source server management, all in one place.</p>
				</div>
				<!-- svelte-ignore a11y_invalid_attribute (placeholder link required until the repository URL exists) -->
				<a href="#" class="shrink-0 text-sm font-semibold text-[var(--accent)] hover:underline">
					GitHub repository
				</a>
			</div>
		</section>

		<section class="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
			<h2 class="font-semibold text-[var(--fg)]">Session</h2>
			<div class="mt-4 flex items-center justify-between gap-6">
				<p class="text-sm text-[var(--fg-muted)]">Sign out of this admin session on this device.</p>
				<button
					type="button"
					onclick={handleLogout}
					disabled={loggingOut}
					class="shrink-0 rounded-xl border border-[var(--border)] px-4 py-2 text-sm font-semibold text-[var(--fg)] transition-colors hover:bg-[var(--track)] disabled:opacity-50"
				>
					{loggingOut ? 'Signing out…' : 'Sign out'}
				</button>
			</div>
		</section>
	</div>
</main>
