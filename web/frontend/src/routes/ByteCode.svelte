<script lang="ts">
    import type { HighlightType } from "../types";

    let {
        code,
        highlight,
        setHighlight,
    }: {
        code: Record<
            string,
            {
                ast_id: number;
                op: string;
            }[]
        >;
        highlight: HighlightType;
        setHighlight: (highlight: HighlightType) => void;
    } = $props();

    const isNotInternal = (entries: [string, object]) => {
        return entries[0] !== "INTERNAL_SETUP";
    };

    const onMouseEnter = (ast_id: number) => {
        setHighlight({ ast_ids: [ast_id], from: 0, to: 0 });
    };

    const onMouseLeave = (e: Event) => {
        e.stopPropagation();
        setHighlight({ ast_ids: [], from: 0, to: 0 });
    };
</script>

{#each Object.entries(code).filter(isNotInternal) as [fn, opCode]}
    <p><b>{fn}</b></p>
    {#each opCode.flatMap(({ ast_id, op }) => op
            .split("\n")
            .filter((op) => op !== "")
            .map((code) => ({ ast_id, op: code }))) as { ast_id, op }}
        <p
            onmouseenter={(e) => {
                e.stopPropagation();
                onMouseEnter(ast_id);
            }}
            class={String(ast_id)}
            style={`font-weight: ${highlight.ast_ids.includes(ast_id) ? "400" : "200"}`}
        >
            {op}
        </p>
    {/each}
{/each}
