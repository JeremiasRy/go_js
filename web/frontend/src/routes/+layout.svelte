<script lang="ts">
	export const ssr = false;
	export const prerender = true;
	import favicon from "$lib/assets/favicon.svg";
	import "../app.css";
	import { setHighlightContext } from "../context";

	let { children } = $props();
	setHighlightContext({from: 0, to:0})
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
