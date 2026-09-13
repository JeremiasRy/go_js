<script lang="ts">
    import type { HighlightStatus } from "../types";

    type PropsType = {
        code: Record<string, { ast_id: number; op: string }[]>;
        setHighlight: (
            opts: {
                astId: number;
                source: "ast" | "op_code";
            } | null,
        ) => void;
        highlight: HighlightStatus | null;
    };
    const { code, setHighlight, highlight }: PropsType = $props();

    const isNotInternalSetup = ([fnName]: [
        string,
        { ast_id: number; op: string }[],
    ]) => {
        return fnName !== "INTERNAL_SETUP";
    };
</script>

<div class="op-code border-solid border-l p-2 mb-1 rounded-md">
    {#each Object.entries(code).filter(isNotInternalSetup) as [fn, operands]}
        <p><b>{fn === "PROGRAM_MAIN" ? "Main" : fn}</b></p>
        {#each operands as { ast_id, op }}
            <p
                class="px-2 whitespace-pre-wrap {Number(
                    ast_id,
                )} {highlight?.astIds.includes(ast_id) ? 'highlight' : ''}"
            >
                {op}
            </p>
        {/each}
    {/each}
</div>

<style>
    .op-code {
        border-color: #e5e7eb;
    }

    .highlight {
        font-weight: 700;
    }
</style>
