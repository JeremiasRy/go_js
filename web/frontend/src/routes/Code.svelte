<script lang="ts">
    import type {
        DisassembledByteCode,
        FunctionName,
        HighlightStatus,
    } from "../types";

    type PropsType = {
        debugInfo: Record<FunctionName, DisassembledByteCode>;
        setHighlight: (
            opts: {
                astId: number;
                source: "ast" | "op_code";
            } | null,
        ) => void;
        highlight: HighlightStatus | null;
    };
    const opElements = new Map<number, HTMLElement>();
    const { debugInfo, setHighlight, highlight }: PropsType = $props();

    const isNotInternalSetup = ([fnName]: [string, DisassembledByteCode]) => {
        return fnName !== "INTERNAL_SETUP";
    };

    $effect(() => {
        if (highlight?.source === "ast") {
            const elements: HTMLElement[] = highlight.astIds
                .filter((n) => opElements.has(n))
                .map((n) => opElements.get(n)) as HTMLElement[];

            const firstElement = elements.reduce<HTMLElement | null>(
                (earliest, current) => {
                    if (!earliest) return current;

                    const position = earliest.compareDocumentPosition(current);
                    if (position & Node.DOCUMENT_POSITION_PRECEDING) {
                        return current;
                    }

                    return earliest;
                },
                null,
            );

            firstElement?.scrollIntoView({
                behavior: "smooth",
                block: "center",
            });
        }
    });
</script>

<div class="op-code border-solid border-l p-2 mb-1 rounded-md">
    {#each Object.entries(debugInfo).filter(isNotInternalSetup) as [fn, debug]}
        <p><b>{fn === "PROGRAM_MAIN" ? "Main" : fn}</b></p>
        {#each debug.byte_code as { op, ast_id }}
            <p
                onmouseover={() => {
                    setHighlight({ astId: ast_id, source: "op_code" });
                }}
                onmouseleave={() => {
                    setHighlight(null);
                }}
                onfocus={() => {}}
                onblur={() => {}}
                bind:this={
                    () => {},
                    (node) => {
                        const previous = opElements.get(ast_id);

                        if (previous === undefined) {
                            opElements.set(ast_id, node);
                            return;
                        }

                        if (
                            previous.compareDocumentPosition(node) &
                            Node.DOCUMENT_POSITION_PRECEDING
                        ) {
                            opElements.set(ast_id, node);
                            return;
                        }
                    }
                }
                class="code cursor-default px-2 whitespace-pre-wrap {Number(
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
        border-color: #3b82f6;
        background-color: #eff6ff;
    }
</style>
