<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/logo.png';
	import Dock from '$lib/components/Dock.svelte';
	import Spotlight from '$lib/components/Spotlight.svelte';
	import ToastHost from '$lib/components/ToastHost.svelte';
	import { getTheme } from '$lib/theme.svelte';
	import { page, updated } from '$app/state';
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
		if (updated.current && typeof window !== 'undefined') {
			window.location.reload();
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
	<div class="launch-screen">
		<div class="launch-content">
			<span class="launch-icon"><img src={favicon} alt="" /></span>
			<p>Opening FaroOS…</p>
		</div>
	</div>
{:else if authenticated}
	<div class="app-frame">
		<div class="app-scroll">
			{@render children()}
		</div>
		<Dock />
		<Spotlight />
	</div>
{/if}
