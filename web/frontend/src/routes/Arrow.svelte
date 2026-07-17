<script lang="ts">
    const { direction }: { direction: "right" | "down" } = $props();
</script>

{#if direction === "down"}
    <span>&#9660</span>
{:else}
    <span>&#x25b6</span>
{/if}
