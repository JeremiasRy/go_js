<script lang="ts">
    import type {
        DisassembledChunk,
        HighlightStatus,
        SetHighlightStatusParams,
    } from "../types";

    type Props = {
        chunk: DisassembledChunk;
        highlight: HighlightStatus | null;
        setHighlight: (opts: SetHighlightStatusParams) => void;
    };

    type OpLine = {
        offset: string;
        text: string;
    };

    type ParsedOp = {
        opcode: OpLine;
        operands: OpLine[];
    };

    const { chunk, highlight, setHighlight }: Props = $props();

    const highlightedAstIds = $derived(
        highlight?.source === "ast"
            ? new Set(highlight.astIds)
            : new Set<number>(),
    );

    const highlightedPointer = $derived(
        highlight !== null && highlight.source !== "ast"
            ? { chunkId: highlight.chunkId, ptr: highlight.opCodePtr }
            : null,
    );

    const rowClasses = $derived(
        chunk.byte_code.map((item) => {
            const byAst = highlightedAstIds.has(item.ast_id);
            const byPointer =
                highlightedPointer !== null &&
                highlightedPointer.chunkId === chunk.id &&
                highlightedPointer.ptr === item.op_ptr;

            return byAst || byPointer ? "highlight" : "";
        }),
    );

    function parseOp(op: string): ParsedOp {
        const lines = op
            .split("\n")
            .filter((line) => line.trim() !== "")
            .map((line): OpLine => {
                const [offset, ...rest] = line.split("|");
                return {
                    offset: offset.trim(),
                    text: rest.join("|").trim(),
                };
            });

        return {
            opcode: lines[0] ?? { offset: "", text: op.trim() },
            operands: lines.slice(1),
        };
    }
</script>

<div class="flex flex-col">
    {#each chunk.byte_code as { op, ast_id, op_ptr }, i}
        {@const parsedOp = parseOp(op)}
        <p
            ondblclick={() => {
                setHighlight({
                    astId: ast_id,
                    source: "op_code",
                    opCodePtr: op_ptr,
                    chunkId: chunk.id,
                });
            }}
            data-code-row
            data-chunk-id={chunk.id}
            data-ast-id={ast_id}
            data-op-ptr={op_ptr}
            class="code cursor-default select-none rounded border-l border-transparent px-2 py-0.5 {Number(
                ast_id,
            )} {rowClasses[i]}"
        >
            <span
                class="grid grid-cols-[3.5rem_minmax(0,1fr)] items-baseline gap-x-2"
            >
                <span class="text-[10px] text-slate-400"
                    >{parsedOp.opcode.offset}</span
                >
                <span
                    class="whitespace-pre-wrap font-semibold text-[#00758D]"
                    >{parsedOp.opcode.text}</span
                >
                {#each parsedOp.operands as operand}
                    <span class="text-[10px] text-slate-400"
                        >{operand.offset}</span
                    >
                    <span class="whitespace-pre-wrap text-slate-500"
                        >{operand.text}</span
                    >
                {/each}
            </span>
        </p>
    {/each}
</div>

<style>
    .code:hover:not(.highlight) {
        border-left-color: #3b82f6;
        background-color: #eff6ff;
    }

    .code.highlight {
        background-color: #eff6ff;
        border-left-color: #3b82f6;
    }

    .code.highlight:hover {
        background-color: #dbeafe;
    }
</style>
