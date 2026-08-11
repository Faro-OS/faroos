<script lang="ts">
	import { authStatus, login, setupAdmin } from '$lib/api';
	import logo from '$lib/assets/logo.png';

	let checking = $state(true);
	let needsSetup = $state(false);
	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);
	let showPassword = $state(false);

	$effect(() => {
		(async () => {
			try {
				const status = await authStatus();
				if (status.authenticated) {
					window.location.href = '/';
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
			error = 'Use at least 8 characters for your password';
			return;
		}

		submitting = true;
		try {
			if (needsSetup) await setupAdmin(username, password);
			else await login(username, password);
			window.location.href = '/';
		} catch {
			if (needsSetup) {
				try {
					const status = await authStatus();
					if (status.authenticated) {
						window.location.href = '/';
						return;
					}
					if (!status.needsSetup) {
						needsSetup = false;
						confirmPassword = '';
						try {
							await login(username, password);
							window.location.href = '/';
							return;
						} catch {
							password = '';
							error = 'The administrator account already exists. Sign in to continue.';
							return;
						}
					}
				} catch {
					// Keep the setup-specific message if status recovery is unavailable.
				}
			}
			error = needsSetup ? 'We could not create the administrator account' : 'That username or password does not look right';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head><title>{needsSetup ? 'Set up' : 'Sign in'} · FaroOS</title></svelte:head>

<main class="relative grid min-h-[100dvh] overflow-hidden bg-[#080a0e] text-white lg:grid-cols-[1.1fr_0.9fr]">
	<section class="relative hidden overflow-hidden lg:block">
		<div class="absolute inset-0" style="background: var(--wallpaper);"></div>
		<div class="absolute -left-32 -top-36 h-[34rem] w-[34rem] rounded-full bg-white/6 blur-[110px]"></div>
		<div class="absolute -bottom-48 right-[-8rem] h-[42rem] w-[42rem] rounded-full bg-white/4 blur-[130px]"></div>
		<div class="absolute inset-0 bg-[linear-gradient(110deg,rgba(255,255,255,0.04)_0%,transparent_32%)]"></div>

		<div class="relative flex h-full min-h-[720px] flex-col justify-between p-12 xl:p-16">
			<div class="flex items-center gap-3"><span class="grid h-11 w-11 place-items-center rounded-[15px] bg-white p-1.5 shadow-xl"><img src={logo} alt="FaroOS" class="h-full w-full object-contain" /></span><div><p class="text-[15px] font-semibold">FaroOS</p><p class="text-[11px] text-white/50">Your servers. One calm place.</p></div></div>

			<div class="max-w-2xl pb-8">
				<p class="mb-5 text-xs font-semibold uppercase tracking-[0.18em] text-white/45">Control without the complexity</p>
				<h1 class="text-5xl font-semibold leading-[1.02] tracking-[-0.06em] xl:text-7xl">Everything running.<br /><span class="text-white/38">Beautifully.</span></h1>
				<p class="mt-6 max-w-lg text-base leading-7 text-white/55">A private control center for every server, container, disk and app you own. Thoughtfully designed, wonderfully fast.</p>
			</div>

			<div class="flex items-center gap-5 text-[11px] text-white/38"><span class="flex items-center gap-2"><span class="h-1.5 w-1.5 rounded-full bg-emerald-400 shadow-[0_0_12px_#34d399]"></span>Local and private</span><span>Encrypted sessions</span><span>Open source</span></div>
		</div>
	</section>

	<section class="relative flex min-h-[100dvh] items-center justify-center bg-[var(--bg)] px-5 py-12 text-[var(--fg)]">
		<div class="absolute left-6 top-6 flex items-center gap-2.5 lg:hidden"><span class="grid h-9 w-9 place-items-center rounded-xl bg-white p-1 shadow-md"><img src={logo} alt="FaroOS" class="h-full w-full object-contain" /></span><span class="text-sm font-semibold">FaroOS</span></div>

		{#if checking}
			<div class="flex flex-col items-center gap-3"><span class="h-5 w-5 animate-spin rounded-full border-2 border-[var(--border-strong)] border-t-[var(--accent)]"></span><p class="text-xs text-[var(--fg-subtle)]">Preparing your space…</p></div>
		{:else}
			<div class="section-enter w-full max-w-[390px]">
				<p class="eyebrow mb-3">{needsSetup ? 'A fresh beginning' : 'Private access'}</p>
				<h2 class="text-3xl font-semibold tracking-[-0.045em] sm:text-[2.15rem]">{needsSetup ? 'Make FaroOS yours.' : 'Welcome back.'}</h2>
				<p class="mb-8 mt-3 text-sm leading-6 text-[var(--fg-subtle)]">{needsSetup ? 'Create the administrator account for this control center. Your data stays on this server.' : 'Sign in to continue to your control center.'}</p>

				<form onsubmit={submit} class="flex flex-col gap-4">
					<label class="flex flex-col gap-2 text-xs font-medium text-[var(--fg-muted)]" for="username">Username
						<input id="username" bind:value={username} required autocomplete="username" placeholder={needsSetup ? 'Choose a username' : 'Your username'} class="control h-12 rounded-[14px] px-4 text-sm text-[var(--fg)] outline-none placeholder:text-[var(--fg-subtle)] focus:border-[var(--accent)]" />
					</label>
					<label class="flex flex-col gap-2 text-xs font-medium text-[var(--fg-muted)]" for="password">Password
						<div class="control flex h-12 items-center rounded-[14px] focus-within:border-[var(--accent)]">
							<input id="password" type={showPassword ? 'text' : 'password'} bind:value={password} required autocomplete={needsSetup ? 'new-password' : 'current-password'} placeholder={needsSetup ? 'At least 8 characters' : 'Your password'} class="min-w-0 flex-1 bg-transparent px-4 text-sm text-[var(--fg)] outline-none placeholder:text-[var(--fg-subtle)]" />
							<button type="button" onclick={() => (showPassword = !showPassword)} class="grid h-10 w-11 place-items-center text-[var(--fg-subtle)] hover:text-[var(--fg)]" aria-label={showPassword ? 'Hide password' : 'Show password'}>
								{#if showPassword}<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 3l18 18M10.6 10.7a2 2 0 0 0 2.7 2.7M9.9 4.2A10.7 10.7 0 0 1 21 12a11.8 11.8 0 0 1-2.2 3.2M6.5 6.5A12 12 0 0 0 3 12c1.8 3.5 5.2 6 9 6 1 0 2-.2 2.9-.5" stroke-linecap="round" stroke-linejoin="round" /></svg>{:else}<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 12s3.5-6 9-6 9 6 9 6-3.5 6-9 6-9-6-9-6Z" /><circle cx="12" cy="12" r="2.5" /></svg>{/if}
							</button>
						</div>
					</label>
					{#if needsSetup}
						<label class="flex flex-col gap-2 text-xs font-medium text-[var(--fg-muted)]" for="confirm-password">Confirm password
							<input id="confirm-password" type={showPassword ? 'text' : 'password'} bind:value={confirmPassword} required autocomplete="new-password" placeholder="Type it once more" class="control h-12 rounded-[14px] px-4 text-sm text-[var(--fg)] outline-none placeholder:text-[var(--fg-subtle)] focus:border-[var(--accent)]" />
						</label>
					{/if}

					{#if error}<div class="flex items-start gap-2 rounded-xl border border-rose-500/15 bg-rose-500/8 px-3.5 py-3 text-xs leading-5 text-rose-500"><span class="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-rose-500"></span>{error}</div>{/if}

					<button type="submit" disabled={submitting} class="primary-control mt-2 flex h-12 items-center justify-center gap-2 rounded-[14px] text-sm font-semibold disabled:opacity-55">
						{submitting ? 'One moment…' : needsSetup ? 'Create my control center' : 'Continue to FaroOS'}
						{#if !submitting}<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2"><path d="m9 18 6-6-6-6" stroke-linecap="round" stroke-linejoin="round" /></svg>{/if}
					</button>
				</form>

				<p class="mt-7 text-center text-[11px] leading-5 text-[var(--fg-subtle)]">Protected by an encrypted, local administrator session.<br />Nothing leaves your infrastructure.</p>
			</div>
		{/if}
	</section>
</main>
