<script lang="ts">
	import { goto } from '$app/navigation';
	import TopBar from '$lib/components/TopBar.svelte';
	import { getTheme, setTheme } from '$lib/theme.svelte';
	import { logout } from '$lib/api';
	import logo from '$lib/assets/logo.png';

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

<main class="section-enter mx-auto w-full max-w-5xl p-4 pb-32 sm:p-7 sm:pb-32 lg:p-10 lg:pb-32">
	<section class="premium-card mb-5 overflow-hidden rounded-[28px] p-5 sm:p-7">
		<div class="flex flex-col gap-6 sm:flex-row sm:items-center sm:justify-between"><div class="flex items-center gap-4"><span class="grid h-16 w-16 shrink-0 place-items-center rounded-[22px] bg-white p-2 shadow-[var(--shadow-md)]"><img src={logo} alt="FaroOS" class="h-full w-full object-contain" /></span><div><p class="eyebrow mb-2">Control center</p><h2 class="text-2xl font-semibold tracking-[-0.04em] text-[var(--fg)]">FaroOS</h2><p class="mt-1 text-sm text-[var(--fg-subtle)]">Open-source server management, beautifully considered.</p></div></div><span class="self-start rounded-full bg-emerald-500/10 px-3 py-1.5 text-[10px] font-semibold text-emerald-500 sm:self-center">Up to date · 0.0.1</span></div>
	</section>

	<div class="grid gap-5 lg:grid-cols-[1.25fr_0.75fr]">
		<div class="space-y-5">
			<section class="surface-card rounded-[24px] p-5 sm:p-6">
				<div class="mb-6"><p class="eyebrow mb-2">Appearance</p><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">Choose your atmosphere</h2><p class="mt-1 text-xs leading-5 text-[var(--fg-subtle)]">FaroOS is tuned for clarity in both bright rooms and late nights.</p></div>
				<div class="grid grid-cols-2 gap-3">
					<button type="button" onclick={() => setTheme('light')} aria-pressed={getTheme() === 'light'} class="group rounded-[20px] border p-2 text-left transition-all {getTheme() === 'light' ? 'border-[var(--accent)] bg-[var(--accent-soft)]' : 'border-[var(--border)] hover:border-[var(--border-strong)]'}">
						<div class="h-28 overflow-hidden rounded-[14px] bg-[#eef0f4] p-3"><div class="mb-2 h-3 w-16 rounded bg-white shadow-sm"></div><div class="grid grid-cols-3 gap-1.5"><div class="col-span-2 h-16 rounded-lg bg-white shadow-sm"></div><div class="h-16 rounded-lg bg-white shadow-sm"></div></div></div><div class="flex items-center justify-between px-1 pb-1 pt-3"><span class="text-xs font-semibold text-[var(--fg)]">Light</span>{#if getTheme() === 'light'}<span class="grid h-5 w-5 place-items-center rounded-full bg-[var(--accent)] text-white"><svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2.5"><path d="m6 12 4 4 8-9" /></svg></span>{/if}</div>
					</button>
					<button type="button" onclick={() => setTheme('dark')} aria-pressed={getTheme() === 'dark'} class="group rounded-[20px] border p-2 text-left transition-all {getTheme() === 'dark' ? 'border-[var(--accent)] bg-[var(--accent-soft)]' : 'border-[var(--border)] hover:border-[var(--border-strong)]'}">
						<div class="h-28 overflow-hidden rounded-[14px] bg-[#0b0e14] p-3"><div class="mb-2 h-3 w-16 rounded bg-[#1d222d]"></div><div class="grid grid-cols-3 gap-1.5"><div class="col-span-2 h-16 rounded-lg bg-[#171b24]"></div><div class="h-16 rounded-lg bg-[#171b24]"></div></div></div><div class="flex items-center justify-between px-1 pb-1 pt-3"><span class="text-xs font-semibold text-[var(--fg)]">Dark</span>{#if getTheme() === 'dark'}<span class="grid h-5 w-5 place-items-center rounded-full bg-[var(--accent)] text-white"><svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2.5"><path d="m6 12 4 4 8-9" /></svg></span>{/if}</div>
					</button>
				</div>
			</section>

			<section class="surface-card overflow-hidden rounded-[24px]">
				<div class="border-b border-[var(--border)] p-5 sm:px-6"><p class="eyebrow mb-2">System</p><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">Environment</h2></div>
				{#each [
					{ label: 'Interface', value: 'FaroOS Web', detail: 'Embedded in the server binary', icon: 'M4 5h16v14H4V5Zm0 4h16' },
					{ label: 'Session security', value: 'Protected', detail: 'HTTP-only administrator session', icon: 'M7 10V8a5 5 0 0 1 10 0v2m-11 0h12v10H6V10Z' },
					{ label: 'Data ownership', value: 'Local', detail: 'Your infrastructure, your database', icon: 'M12 3 4 7v5c0 5 3.4 8 8 9 4.6-1 8-4 8-9V7l-8-4Z' }
				] as item, index (item.label)}
					<div class="flex items-center gap-4 p-5 sm:px-6 {index > 0 ? 'border-t border-[var(--border)]' : ''}"><span class="grid h-10 w-10 shrink-0 place-items-center rounded-[14px] bg-[var(--track)] text-[var(--fg-muted)]"><svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.7"><path d={item.icon} stroke-linecap="round" stroke-linejoin="round" /></svg></span><div class="min-w-0 flex-1"><p class="text-sm font-medium text-[var(--fg)]">{item.label}</p><p class="mt-0.5 text-[11px] text-[var(--fg-subtle)]">{item.detail}</p></div><span class="text-xs font-medium text-[var(--fg-muted)]">{item.value}</span></div>
				{/each}
			</section>
		</div>

		<div class="space-y-5">
			<section class="surface-card rounded-[24px] p-5 sm:p-6"><p class="eyebrow mb-3">Project</p><h2 class="text-lg font-semibold tracking-tight text-[var(--fg)]">Built in the open.</h2><p class="mt-2 text-xs leading-5 text-[var(--fg-subtle)]">FaroOS is free software, built for people who want their cloud to truly belong to them.</p><a href="https://github.com/Faro-OS/faroos" target="_blank" rel="noreferrer" class="control mt-5 flex h-10 items-center justify-center gap-2 rounded-xl text-xs font-semibold text-[var(--fg-muted)]">View on GitHub <svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2"><path d="M7 17 17 7M8 7h9v9" /></svg></a></section>
			<section class="surface-card rounded-[24px] p-5 sm:p-6"><span class="mb-4 grid h-10 w-10 place-items-center rounded-[14px] bg-rose-500/10 text-rose-500"><svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M10 5H5v14h5M14 8l4 4-4 4M18 12H9" stroke-linecap="round" stroke-linejoin="round" /></svg></span><h2 class="text-sm font-semibold text-[var(--fg)]">End this session</h2><p class="mt-1 text-xs leading-5 text-[var(--fg-subtle)]">Sign out securely on this device. Your servers keep running uninterrupted.</p><button type="button" onclick={handleLogout} disabled={loggingOut} class="mt-5 w-full rounded-xl border border-rose-500/15 bg-rose-500/8 px-4 py-2.5 text-xs font-semibold text-rose-500 transition-colors hover:bg-rose-500/12 disabled:opacity-50">{loggingOut ? 'Signing out…' : 'Sign out'}</button></section>
		</div>
	</div>
</main>
