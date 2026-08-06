<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import { getTheme } from '$lib/theme.svelte';

	let { children } = $props();

	$effect(() => {
		if (typeof document !== 'undefined') {
			document.documentElement.dataset.theme = getTheme();
		}
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>FaroOS</title>
</svelte:head>

<div class="flex h-screen w-screen overflow-hidden">
	<Sidebar />
	<div class="flex flex-1 flex-col overflow-y-auto">
		{@render children()}
	</div>
</div>
