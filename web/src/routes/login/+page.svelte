<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStatus, login, setupAdmin } from '$lib/api';

	let checking = $state(true);
	let needsSetup = $state(false);
	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		(async () => {
			try {
				const status = await authStatus();
				if (status.authenticated) {
					await goto('/');
					return;
				}
				needsSetup = status.needsSetup;
			} finally {
				checking = false;
			}
		})();
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = null;

		if (needsSetup && password !== confirmPassword) {
			error = "Passwords don't match";
			return;
		}
		if (needsSetup && password.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}

		submitting = true;
		try {
			if (needsSetup) {
				await setupAdmin(username, password);
			} else {
				await login(username, password);
			}
			await goto('/');
		} catch {
			error = needsSetup ? 'Could not create the admin account' : 'Invalid username or password';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="grid h-screen w-screen place-items-center bg-[var(--bg)] px-4">
	{#if checking}
		<p class="text-[var(--fg-subtle)]">Loading…</p>
	{:else}
		<div class="w-full max-w-sm rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-sm">
			<div class="mb-6 flex items-center gap-2.5">
				<span class="grid h-9 w-9 place-items-center rounded-xl bg-[var(--accent)] text-[var(--accent-fg)]">
					<svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M12 2 3 7v6c0 5 4 8.5 9 9 5-.5 9-4 9-9V7l-9-5Z" stroke-linecap="round" stroke-linejoin="round" />
						<path d="M12 8v5l3 2" stroke-linecap="round" stroke-linejoin="round" />
					</svg>
				</span>
				<span class="text-xl font-semibold tracking-tight text-[var(--fg)]">FaroOS</span>
			</div>

			<h1 class="mb-1 text-lg font-semibold text-[var(--fg)]">
				{needsSetup ? 'Create the admin account' : 'Sign in'}
			</h1>
			<p class="mb-5 text-sm text-[var(--fg-subtle)]">
				{needsSetup ? 'This is the first run — set up the account that controls this panel.' : 'Welcome back.'}
			</p>

			<form onsubmit={submit} class="flex flex-col gap-3">
				<label class="flex flex-col gap-1.5 text-sm">
					<span class="font-medium text-[var(--fg-muted)]">Username</span>
					<input
						bind:value={username}
						required
						autocomplete="username"
						class="rounded-xl border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-[var(--fg)] outline-none focus:border-[var(--accent)]"
					/>
				</label>
				<label class="flex flex-col gap-1.5 text-sm">
					<span class="font-medium text-[var(--fg-muted)]">Password</span>
					<input
						type="password"
						bind:value={password}
						required
						autocomplete={needsSetup ? 'new-password' : 'current-password'}
						class="rounded-xl border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-[var(--fg)] outline-none focus:border-[var(--accent)]"
					/>
				</label>
				{#if needsSetup}
					<label class="flex flex-col gap-1.5 text-sm">
						<span class="font-medium text-[var(--fg-muted)]">Confirm password</span>
						<input
							type="password"
							bind:value={confirmPassword}
							required
							autocomplete="new-password"
							class="rounded-xl border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-[var(--fg)] outline-none focus:border-[var(--accent)]"
						/>
					</label>
				{/if}

				{#if error}
					<p class="text-sm text-rose-500">{error}</p>
				{/if}

				<button
					type="submit"
					disabled={submitting}
					class="mt-2 rounded-xl bg-[var(--accent)] px-4 py-2.5 text-sm font-semibold text-[var(--accent-fg)] disabled:opacity-50"
				>
					{submitting ? 'Please wait…' : needsSetup ? 'Create account & sign in' : 'Sign in'}
				</button>
			</form>
		</div>
	{/if}
</div>
