<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/logo.png';
	import Dock from '$lib/components/Dock.svelte';
	import ToastHost from '$lib/components/ToastHost.svelte';
	import { getTheme } from '$lib/theme.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { authStatus } from '$lib/api';

	let { children } = $props();

	let authChecked = $state(false);
	let authenticated = $state(false);

	const isLoginRoute = $derived(page.url.pathname === '/login');

	$effect(() => {
		if (typeof document !== 'undefined') {
			document.documentElement.dataset.theme = getTheme();
		}
	});

	$effect(() => {
		(async () => {
			try {
				const status = await authStatus();
				authenticated = status.authenticated;
				if (!status.authenticated && page.url.pathname !== '/login') {
					await goto('/login');
				}
			} catch {
				authenticated = false;
			} finally {
				authChecked = true;
			}
		})();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>FaroOS</title>
</svelte:head>

<ToastHost />

{#if isLoginRoute}
	{@render children()}
{:else if !authChecked}
	<div class="grid h-screen w-screen place-items-center bg-[var(--bg)]">
		<p class="text-[var(--fg-subtle)]">Loading…</p>
	</div>
{:else if authenticated}
	<div class="h-screen w-screen overflow-y-auto pb-24">
		{@render children()}
	</div>
	<Dock />
{/if}
