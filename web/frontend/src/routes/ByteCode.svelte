<script module lang="ts">
    const _hoverState: { [key: number]: Boolean } = $state({});
</script>

<script lang="ts">
    import type { HighlightStatus } from "../types";

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
        highlight: HighlightStatus | null;
        setHighlight: (
            opts: {
                astId: number;
                source: "ast" | "op_code";
            } | null,
        ) => void;
    } = $props();

    const isNotInternal = (entries: [string, object]) => {
        return entries[0] !== "INTERNAL_SETUP";
    };

    const onMouseEnter = (e: Event, astId: number) => {
        e.stopPropagation();
        setHighlight({ astId, source: "op_code" });
    };

    const onMouseLeave = (e: Event) => {
        e.stopPropagation();
        setHighlight(null);
    };
</script>

{#each Object.entries(code).filter(isNotInternal) as [fn, opCode]}
    <p><b>{fn}</b></p>
    {#each opCode.flatMap(({ ast_id, op }) => op
            .split("\n")
            .filter((op) => op !== "")
            .map((code) => ({ ast_id, op: code }))) as { ast_id, op }}
        <p
            onmouseover={(e) => {
                e.currentTarget.style = "font-weight: 400;";
                onMouseEnter(e, ast_id);
            }}
            onmouseout={(e) => {
                e.stopPropagation();
                e.currentTarget.style = "font-weight: 200;";
                setHighlight(null);
            }}
            onfocus={() => {}}
            onblur={() => {}}
            style={`font-weight: ${
                highlight !== null &&
                highlight.source === "ast" &&
                highlight.astIds.includes(ast_id)
                    ? "400"
                    : "200"
            }`}
        >
            {op}
        </p>
    {/each}
{/each}
