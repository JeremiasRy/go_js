<script lang="ts">
    import type {
        DisassembledByteCode,
        FunctionName,
        HighlightStatus,
        SetHighlightStatusParams,
    } from "../types";

    type PropsType = {
        debugInfo: Record<FunctionName, DisassembledByteCode>;
        setHighlight: (opts: SetHighlightStatusParams) => void;
        highlight: HighlightStatus | null;
    };
    const byteOpElements = new Map<number, HTMLElement>();
    const byteOpByPtr = new Map<string, HTMLElement>();
    const asmOpElements = new Map<number, HTMLElement>();
    const asmOpByPtr = new Map<string, HTMLElement>();
    const { debugInfo, setHighlight, highlight }: PropsType = $props();

    const ptrKey = (fn: string, ptr: number) => `${fn}:${ptr}`;

    const earliestElement = (elements: HTMLElement[]) => {
        return elements.reduce<HTMLElement | null>((earliest, current) => {
            if (!earliest) return current;

            const position = earliest.compareDocumentPosition(current);
            if (position & Node.DOCUMENT_POSITION_PRECEDING) {
                return current;
            }

            return earliest;
        }, null);
    };

    const scrollTo = (element: HTMLElement | null | undefined) => {
        element?.scrollIntoView({
            behavior: "smooth",
            block: "center",
        });
    };

    const isNotInternalSetup = ([fnName]: [string, DisassembledByteCode]) => {
        return fnName !== "INTERNAL_SETUP";
    };

    $effect(() => {
        if (highlight === null) {
            return;
        }

        const byteCodeByAst = earliestElement(
            highlight.astIds
                .filter((n) => byteOpElements.has(n))
                .map((n) => byteOpElements.get(n) as HTMLElement),
        );
        const asmByAst = earliestElement(
            highlight.astIds
                .filter((n) => asmOpElements.has(n))
                .map((n) => asmOpElements.get(n) as HTMLElement),
        );

        if (highlight.source === "ast") {
            scrollTo(byteCodeByAst);
            scrollTo(asmByAst);
            return;
        }

        if (highlight.source === "op_code") {
            scrollTo(
                asmOpByPtr.get(ptrKey(highlight.fn, highlight.opCodePtr)) ??
                    asmByAst,
            );
            return;
        }

        if (highlight.source === "asm") {
            scrollTo(
                byteOpByPtr.get(ptrKey(highlight.fn, highlight.opCodePtr)) ??
                    byteCodeByAst,
            );
        }
    });

    function formatAssemblyInstruction(inst: string) {
        let [offset, ...rest] = inst.split(" ");

        offset = offset.replace(/^0+/, "");
        offset = offset.padStart(4, "0");

        return [offset, "|", ...rest].join(" ");
    }

    function determineHiglightStatusForOpCode({
        op_ptr,
        ast_id,
        fn,
    }: {
        op_ptr: number;
        ast_id: number;
        fn: string;
    }) {
        if (
            (highlight?.source === "asm" || highlight?.source === "op_code") &&
            fn === highlight.fn &&
            op_ptr === highlight.opCodePtr
        ) {
            return "highlight";
        }

        if (highlight?.source === "ast" && highlight?.astIds.includes(ast_id)) {
            return "highlight";
        }

        return "";
    }

    function determineHighlightStatusForAsm({
        ast_id,
        op_code_ptr,
        fn,
    }: {
        ast_id: number;
        op_code_ptr: number;
        fn: string;
    }) {
        if (
            (highlight?.source === "asm" || highlight?.source === "op_code") &&
            highlight?.opCodePtr === op_code_ptr &&
            highlight?.fn === fn
        ) {
            return "highlight";
        }

        if (highlight?.source === "ast" && highlight?.astIds.includes(ast_id)) {
            return "highlight";
        }
    }
</script>

<div class="flex flex-1 min-h-0 flex-col gap-2 overflow-y-auto">
    {#each Object.entries(debugInfo).filter(isNotInternalSetup) as [fn, { byte_code, jit }]}
        <div class="rounded-lg border border-slate-200 bg-white p-2">
            <p class="mb-2 text-sm font-semibold text-slate-600">
                {fn === "PROGRAM_MAIN" ? "Main" : fn}
            </p>
            <div class="flex flex-row w-full gap-2 max-h-[800px]">
                <div
                    class="flex-1 min-w-0 overflow-hidden overflow-y-auto rounded-md border border-slate-200 bg-slate-50 p-1.5"
                >
                    {#each byte_code as { op, ast_id, op_ptr }, i}
                        {@const isHighlighted =
                            determineHiglightStatusForOpCode({
                                op_ptr,
                                ast_id,
                                fn,
                            }) === "highlight"}
                        {@const previousHighlighted =
                            i > 0 &&
                            determineHiglightStatusForOpCode({
                                op_ptr: byte_code[i - 1].op_ptr,
                                ast_id: byte_code[i - 1].ast_id,
                                fn,
                            }) === "highlight"}
                        {@const nextHighlighted =
                            i < byte_code.length - 1 &&
                            determineHiglightStatusForOpCode({
                                op_ptr: byte_code[i + 1].op_ptr,
                                ast_id: byte_code[i + 1].ast_id,
                                fn,
                            }) === "highlight"}
                        {@const isHighlightStart =
                            isHighlighted && !previousHighlighted}
                        {@const isHighlightEnd =
                            isHighlighted && !nextHighlighted}
                        <p
                            ondblclick={() => {
                                setHighlight({
                                    astId: ast_id,
                                    source: "op_code",
                                    opCodePtr: op_ptr,
                                    fn,
                                });
                            }}
                            bind:this={
                                () => {},
                                (node) => {
                                    byteOpByPtr.set(ptrKey(fn, op_ptr), node);
                                    const previous = byteOpElements.get(ast_id);

                                    if (previous === undefined) {
                                        byteOpElements.set(ast_id, node);
                                        return;
                                    }

                                    if (
                                        previous.compareDocumentPosition(node) &
                                        Node.DOCUMENT_POSITION_PRECEDING
                                    ) {
                                        byteOpElements.set(ast_id, node);
                                        return;
                                    }
                                }
                            }
                            class="code cursor-default select-none rounded border border-transparent px-2 py-0.5 whitespace-pre-wrap transition-colors {Number(
                                ast_id,
                            )} {isHighlighted ? 'highlight' : ''} {isHighlightStart
                                ? 'highlight-start'
                                : ''} {isHighlightEnd
                                ? 'highlight-end'
                                : ''}"
                        >
                            {op}
                        </p>
                    {/each}
                </div>
                {#if jit.length > 0}
                    <div
                        class="flex-1 min-w-0 overflow-hidden overflow-y-auto rounded-md border border-slate-200 bg-slate-50 p-1.5"
                    >
                        {#each jit as { inst, op_code_ptr, ast_id }, i}
                            {@const isHighlighted =
                                determineHighlightStatusForAsm({
                                    ast_id,
                                    fn,
                                    op_code_ptr,
                                }) === "highlight"}
                            {@const previousHighlighted =
                                i > 0 &&
                                determineHighlightStatusForAsm({
                                    ast_id: jit[i - 1].ast_id,
                                    fn,
                                    op_code_ptr: jit[i - 1].op_code_ptr,
                                }) === "highlight"}
                            {@const nextHighlighted =
                                i < jit.length - 1 &&
                                determineHighlightStatusForAsm({
                                    ast_id: jit[i + 1].ast_id,
                                    fn,
                                    op_code_ptr: jit[i + 1].op_code_ptr,
                                }) === "highlight"}
                            {@const isHighlightStart =
                                isHighlighted && !previousHighlighted}
                            {@const isHighlightEnd =
                                isHighlighted && !nextHighlighted}
                            <p
                                ondblclick={() => {
                                    setHighlight({
                                        astId: ast_id,
                                        source: "asm",
                                        opCodePtr: op_code_ptr,
                                        fn,
                                    });
                                }}
                                bind:this={
                                    () => {},
                                    (node) => {
                                        asmOpByPtr.set(ptrKey(fn, op_code_ptr), node);
                                        const previous = asmOpElements.get(ast_id);

                                        if (previous === undefined) {
                                            asmOpElements.set(ast_id, node);
                                            return;
                                        }

                                        if (
                                            previous.compareDocumentPosition(
                                                node,
                                            ) &
                                            Node.DOCUMENT_POSITION_PRECEDING
                                        ) {
                                            asmOpElements.set(ast_id, node);
                                            return;
                                        }
                                    }
                                }
                                class="code cursor-default select-none rounded border border-transparent px-2 py-0.5 whitespace-pre-wrap transition-colors ast-{Number(
                                    ast_id,
                                )} op-{Number(
                                    op_code_ptr,
                                )} {isHighlighted ? 'highlight' : ''} {isHighlightStart
                                    ? 'highlight-start'
                                    : ''} {isHighlightEnd
                                    ? 'highlight-end'
                                    : ''}"
                            >
                                {formatAssemblyInstruction(inst)}
                            </p>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    {/each}
</div>

<style>
    .code:hover:not(.highlight) {
        border-color: #3b82f6;
        background-color: #eff6ff;
    }

    .code.highlight {
        background-color: #eff6ff;
        border-top-color: transparent;
        border-bottom-color: transparent;
        border-left-color: #3b82f6;
        border-right-color: #3b82f6;
        border-radius: 0;
    }

    .code.highlight-start {
        border-top-color: #3b82f6;
        border-top-left-radius: 0.25rem;
        border-top-right-radius: 0.25rem;
    }

    .code.highlight-end {
        border-bottom-color: #3b82f6;
        border-bottom-left-radius: 0.25rem;
        border-bottom-right-radius: 0.25rem;
    }

    .code.highlight-start.highlight-end {
        border-radius: 0.25rem;
    }

    .code.highlight:hover {
        background-color: #dbeafe;
    }
</style>
