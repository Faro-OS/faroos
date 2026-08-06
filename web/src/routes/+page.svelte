<script lang="ts">
	import { getSection } from '$lib/section.svelte';
	import DashboardSection from '$lib/sections/DashboardSection.svelte';
	import ServersSection from '$lib/sections/ServersSection.svelte';
	import ContainersSection from '$lib/sections/ContainersSection.svelte';
	import StorageSection from '$lib/sections/StorageSection.svelte';
	import AppsSection from '$lib/sections/AppsSection.svelte';
	import TerminalSection from '$lib/sections/TerminalSection.svelte';
	import FilesSection from '$lib/sections/FilesSection.svelte';
	import SettingsSection from '$lib/sections/SettingsSection.svelte';
</script>

{#if getSection() === 'dashboard'}
	<DashboardSection />
{:else if getSection() === 'servers'}
	<ServersSection />
{:else if getSection() === 'containers'}
	<ContainersSection />
{:else if getSection() === 'storage'}
	<StorageSection />
{:else if getSection() === 'apps'}
	<AppsSection />
{:else if getSection() === 'terminal'}
	<TerminalSection />
{:else if getSection() === 'files'}
	<FilesSection />
{:else if getSection() === 'settings'}
	<SettingsSection />
{/if}
