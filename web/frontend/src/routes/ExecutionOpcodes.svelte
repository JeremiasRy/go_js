<script lang="ts">
    import { tick } from "svelte";
    import type {
        DisassembledChunk,
        HighlightStatus,
        SetHighlightStatusParams,
        VMState,
    } from "../types";
    import OpcodeList from "./OpcodeList.svelte";

    type Props = {
        chunks: DisassembledChunk[];
        highlight: HighlightStatus | null;
        setHighlight: (opts: SetHighlightStatusParams) => void;
        vm: VMState | null;
    };

    const { chunks, highlight, setHighlight, vm }: Props = $props();

    let scrollContainer = $state<HTMLDivElement | undefined>(undefined);

    const chunkEntries = $derived(
        chunks
            .filter((chunk) => !chunk.hidden)
            .sort((a, b) => {
                if (a.name !== b.name) {
                    return a.name < b.name ? -1 : 1;
                }
                return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
            }),
    );

    $effect(() => {
        const current = highlight;
        if (
            current === null ||
            current.source === "ast" ||
            scrollContainer === undefined
        ) {
            return;
        }

        void tick().then(() => {
            const row = scrollContainer?.querySelector<HTMLElement>(
                `[data-chunk-id="${current.chunkId}"][data-op-ptr="${current.opCodePtr}"]`,
            );
            row?.scrollIntoView({
                behavior: "auto",
                block: "center",
            });
        });
    });
</script>

<div
    bind:this={scrollContainer}
    class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-2"
>
    {#each chunkEntries as chunk (chunk.id)}
        <section>
            <header class="border-b border-slate-100 bg-slate-50/70 px-2 py-1">
                <span
                    class="block truncate font-mono text-[10px] font-semibold text-slate-600"
                >
                    {chunk.name}-{chunk.id}
                </span>
            </header>
            <div class="p-1">
                <OpcodeList {chunk} {highlight} {setHighlight} />
            </div>
        </section>
    {/each}
</div>
