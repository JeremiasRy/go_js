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

<div
    class="op-code border-solid border-l p-2 mb-1 rounded-md overflow-hidden overflow-y-auto"
>
    {#each Object.entries(debugInfo).filter(isNotInternalSetup) as [fn, { byte_code, jit }]}
        <p><b>{fn === "PROGRAM_MAIN" ? "Main" : fn}</b></p>
        <div class="flex flex-row w-full max-h-[800px]">
            <div class="w-1/2 overflow-hidden overflow-y-auto">
                {#each byte_code as { op, ast_id, op_ptr }}
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
                        class="code cursor-default select-none px-2 whitespace-pre-wrap {Number(
                            ast_id,
                        )} {determineHiglightStatusForOpCode({
                            op_ptr,
                            ast_id,
                            fn,
                        })}"
                    >
                        {op}
                    </p>
                {/each}
            </div>
            <div class="w-1/2 overflow-hidden overflow-y-auto">
                {#each jit as { inst, op_code_ptr, ast_id }}
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
                                    previous.compareDocumentPosition(node) &
                                    Node.DOCUMENT_POSITION_PRECEDING
                                ) {
                                    asmOpElements.set(ast_id, node);
                                    return;
                                }
                            }
                        }
                        class="code cursor-default select-none px-2 whitespace-pre-wrap ast-{Number(
                            ast_id,
                        )} op-{Number(
                            op_code_ptr,
                        )} {determineHighlightStatusForAsm({
                            ast_id,
                            fn,
                            op_code_ptr,
                        })}"
                    >
                        {formatAssemblyInstruction(inst)}
                    </p>
                {/each}
            </div>
        </div>
    {/each}
</div>

<style>
    .op-code {
        border-color: #e5e7eb;
    }

    .code:hover {
        border-color: #3b82f6;
        background-color: #eff6ff;
    }

    .highlight {
        border-color: #3b82f6;
        background-color: #eff6ff;
    }
</style>
