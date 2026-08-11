<script lang="ts">
	import { iconGradient, iconInitial } from '$lib/iconStyle';

	let { name, icon, size = 56 }: { name: string; icon?: string; size?: number } = $props();

	let failedIcon = $state<string | undefined>();
</script>

{#if icon && failedIcon !== icon}
	<img
		src={icon}
		alt=""
		loading="lazy"
		onerror={() => (failedIcon = icon)}
		style="width: {size}px; height: {size}px;"
		class="shrink-0 rounded-[22%] object-cover shadow-sm"
	/>
{:else}
	<div
		style="width: {size}px; height: {size}px; background: {iconGradient(name)}; font-size: {size * 0.42}px;"
		class="grid shrink-0 place-items-center rounded-[22%] font-semibold text-white shadow-sm"
	>
		{iconInitial(name)}
	</div>
{/if}
