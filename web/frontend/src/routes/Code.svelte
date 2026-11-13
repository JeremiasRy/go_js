<script lang="ts">
    import { tick, untrack } from "svelte";
    import { slide } from "svelte/transition";
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

    const MAIN_FUNCTION = "PROGRAM_MAIN";
    const EXPAND_DURATION = 200;

    let expandedFunctions = $state<string[]>([]);

    const sameFunctions = (a: string[], b: string[]) => {
        return a.length === b.length && a.every((fn) => b.includes(fn));
    };

    function toggleFunction(fn: string) {
        expandedFunctions = expandedFunctions.includes(fn)
            ? expandedFunctions.filter((name) => name !== fn)
            : [...expandedFunctions, fn];
    }

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

    let codeScrollContainer: HTMLElement;

    const clamp = (value: number, min: number, max: number) => {
        return Math.min(Math.max(value, min), max);
    };

    const getScrollContainer = (element: HTMLElement) => {
        return element.closest<HTMLElement>("[data-code-scroll]");
    };

    const getTargetTop = (container: HTMLElement, target: HTMLElement) => {
        const containerRect = container.getBoundingClientRect();
        const targetRect = target.getBoundingClientRect();

        return targetRect.top - containerRect.top + container.scrollTop;
    };

    const getMaxScroll = (container: HTMLElement) => {
        return Math.max(0, container.scrollHeight - container.clientHeight);
    };

    function rebuildElementMaps() {
        byteOpElements.clear();
        byteOpByPtr.clear();
        asmOpElements.clear();
        asmOpByPtr.clear();

        if (!codeScrollContainer) {
            return;
        }

        const expanded = new Set(expandedFunctions);

        for (const row of codeScrollContainer.querySelectorAll<HTMLElement>(
            "[data-byte-row]",
        )) {
            const fn = row.dataset.fn;
            if (fn === undefined || !expanded.has(fn)) {
                continue;
            }

            const astId = Number(row.dataset.astId);
            const key = ptrKey(fn, Number(row.dataset.opPtr));

            if (!byteOpByPtr.has(key)) {
                byteOpByPtr.set(key, row);
            }

            if (!byteOpElements.has(astId)) {
                byteOpElements.set(astId, row);
            }
        }

        for (const row of codeScrollContainer.querySelectorAll<HTMLElement>(
            "[data-jit-row]",
        )) {
            const fn = row.dataset.fn;
            if (fn === undefined || !expanded.has(fn)) {
                continue;
            }

            const astId = Number(row.dataset.astId);
            const key = ptrKey(fn, Number(row.dataset.opCodePtr));

            if (!asmOpByPtr.has(key)) {
                asmOpByPtr.set(key, row);
            }

            if (!asmOpElements.has(astId)) {
                asmOpElements.set(astId, row);
            }
        }
    }

    function scrollRootToTarget(
        column: HTMLElement,
        target: HTMLElement,
        commonOffset: number,
    ) {
        if (!codeScrollContainer) {
            return;
        }

        const rootRect = codeScrollContainer.getBoundingClientRect();
        const columnRect = column.getBoundingClientRect();
        const columnTopInRoot =
            columnRect.top - rootRect.top + codeScrollContainer.scrollTop;
        const targetTopInRoot = columnTopInRoot + commonOffset;
        const targetHeight = target.getBoundingClientRect().height;
        const rootTarget =
            targetTopInRoot +
            targetHeight / 2 -
            codeScrollContainer.clientHeight / 2;

        codeScrollContainer.scrollTo({
            top: rootTarget,
            behavior: "smooth",
        });
    }

    function alignColumns(bytecodeTarget: HTMLElement, jitTarget: HTMLElement) {
        const bytecodeScroll = getScrollContainer(bytecodeTarget);
        const jitScroll = getScrollContainer(jitTarget);

        if (
            !bytecodeScroll ||
            !jitScroll ||
            bytecodeScroll.parentElement !== jitScroll.parentElement
        ) {
            scrollTo(earliestElement([bytecodeTarget, jitTarget]));
            return;
        }

        const bytecodeTop = getTargetTop(bytecodeScroll, bytecodeTarget);
        const jitTop = getTargetTop(jitScroll, jitTarget);
        const bytecodeMaxScroll = getMaxScroll(bytecodeScroll);
        const jitMaxScroll = getMaxScroll(jitScroll);

        const lower = Math.max(
            bytecodeTop - bytecodeMaxScroll,
            jitTop - jitMaxScroll,
        );
        const upper = Math.min(bytecodeTop, jitTop);
        const preferred =
            Math.min(bytecodeScroll.clientHeight, jitScroll.clientHeight) / 2;
        const commonOffset =
            lower <= upper ? clamp(preferred, lower, upper) : (lower + upper) / 2;

        bytecodeScroll.scrollTo({
            top: clamp(bytecodeTop - commonOffset, 0, bytecodeMaxScroll),
            behavior: "smooth",
        });
        jitScroll.scrollTo({
            top: clamp(jitTop - commonOffset, 0, jitMaxScroll),
            behavior: "smooth",
        });

        scrollRootToTarget(bytecodeScroll, bytecodeTarget, commonOffset);
    }

    const isNotInternalSetup = ([fnName]: [string, DisassembledByteCode]) => {
        return fnName !== "INTERNAL_SETUP";
    };

    const functionEntries = $derived(
        Object.entries(debugInfo).filter(isNotInternalSetup),
    );

    const astToFunctions = $derived.by(() => {
        const map = new Map<number, Set<string>>();

        const add = (astId: number, fn: string) => {
            let functions = map.get(astId);
            if (functions === undefined) {
                functions = new Set<string>();
                map.set(astId, functions);
            }
            functions.add(fn);
        };

        for (const [fn, { byte_code, jit }] of functionEntries) {
            for (const { ast_id } of byte_code) {
                add(ast_id, fn);
            }

            for (const { ast_id } of jit) {
                add(ast_id, fn);
            }
        }

        return map;
    });

    const affectedFunctions = $derived.by(() => {
        if (highlight === null) {
            return [];
        }

        if (highlight.source !== "ast") {
            return functionEntries.some(([fn]) => fn === highlight.fn)
                ? [highlight.fn]
                : [MAIN_FUNCTION];
        }

        const affected = new Set<string>();

        for (const astId of highlight.astIds) {
            for (const fn of astToFunctions.get(astId) ?? []) {
                affected.add(fn);
            }
        }

        return affected.size > 0 ? [...affected] : [MAIN_FUNCTION];
    });

    function scrollToHighlight(currentHighlight: HighlightStatus | null) {
        if (currentHighlight === null) {
            return;
        }

        rebuildElementMaps();

        let bytecodeTarget: HTMLElement | null = null;
        let jitTarget: HTMLElement | null = null;

        if (currentHighlight.source === "ast") {
            bytecodeTarget = earliestElement(
                currentHighlight.astIds
                    .map((n) => byteOpElements.get(n))
                    .filter(
                        (element): element is HTMLElement =>
                            element !== undefined &&
                            element.offsetParent !== null,
                    ),
            );
            jitTarget = earliestElement(
                currentHighlight.astIds
                    .map((n) => asmOpElements.get(n))
                    .filter(
                        (element): element is HTMLElement =>
                            element !== undefined &&
                            element.offsetParent !== null,
                    ),
            );
        } else {
            const key = ptrKey(currentHighlight.fn, currentHighlight.opCodePtr);
            bytecodeTarget = byteOpByPtr.get(key) ?? null;
            jitTarget = asmOpByPtr.get(key) ?? null;
        }

        if (bytecodeTarget && !bytecodeTarget.offsetParent) {
            bytecodeTarget = null;
        }

        if (jitTarget && !jitTarget.offsetParent) {
            jitTarget = null;
        }

        if (bytecodeTarget && jitTarget) {
            alignColumns(bytecodeTarget, jitTarget);
            return;
        }

        scrollTo(bytecodeTarget ?? jitTarget);
    }

    $effect(() => {
        const currentHighlight = highlight;
        const affected = affectedFunctions;
        const previous = untrack(() => expandedFunctions);
        const expansionChanged = !sameFunctions(previous, affected);

        expandedFunctions = [...affected];

        let cancelled = false;
        let timeout: ReturnType<typeof setTimeout> | undefined;

        tick().then(() => {
            if (cancelled || currentHighlight === null) {
                return;
            }

            if (expansionChanged) {
                timeout = setTimeout(() => {
                    if (!cancelled) {
                        scrollToHighlight(currentHighlight);
                    }
                }, EXPAND_DURATION + 20);
            } else {
                scrollToHighlight(currentHighlight);
            }
        });

        return () => {
            cancelled = true;
            if (timeout !== undefined) {
                clearTimeout(timeout);
            }
        };
    });

    function formatAssemblyInstruction(inst: string) {
        let [offset, ...rest] = inst.split(" ");

        offset = offset.replace(/^0+/, "");
        offset = offset.padStart(4, "0");

        return [offset, "|", ...rest].join(" ");
    }

    const highlightedAstIds = $derived(
        highlight?.source === "ast"
            ? new Set(highlight.astIds)
            : new Set<number>(),
    );

    const highlightedPointer = $derived(
        highlight !== null && highlight.source !== "ast"
            ? { fn: highlight.fn, ptr: highlight.opCodePtr }
            : null,
    );

    function buildHighlightClasses<T extends { ast_id: number }>({
        items,
        fn,
        getPtr,
    }: {
        items: T[];
        fn: string;
        getPtr: (item: T) => number;
    }) {
        return items.map((item) => {
            const byAst = highlightedAstIds.has(item.ast_id);
            const byPointer =
                highlightedPointer !== null &&
                highlightedPointer.fn === fn &&
                highlightedPointer.ptr === getPtr(item);

            return byAst || byPointer ? "highlight" : "";
        });
    }

    const getOpCodePtr = (item: { op_ptr: number }) => item.op_ptr;
    const getJitPtr = (item: { op_code_ptr: number }) => item.op_code_ptr;
</script>

<div
    bind:this={codeScrollContainer}
    class="flex flex-1 min-h-0 flex-col gap-2 overflow-y-auto"
>
    {#each functionEntries as [fn, { byte_code, jit }], fnIndex}
        {@const isExpanded = expandedFunctions.includes(fn)}
        {@const byteHighlightClasses = buildHighlightClasses({
            items: byte_code,
            fn,
            getPtr: getOpCodePtr,
        })}
        {@const jitHighlightClasses = buildHighlightClasses({
            items: jit,
            fn,
            getPtr: getJitPtr,
        })}
        <div class="rounded-lg border border-slate-200 bg-white p-2">
            <button
                type="button"
                class="flex w-full cursor-pointer items-center justify-between gap-2 text-left text-sm font-semibold text-slate-600 hover:text-slate-800"
                aria-expanded={isExpanded}
                aria-controls={`bytecode-panel-${fnIndex}`}
                onclick={() => toggleFunction(fn)}
            >
                <span class="flex min-w-0 items-center gap-2">
                    <span class="truncate">
                        {fn === MAIN_FUNCTION ? "Main" : fn}
                    </span>
                    {#if jit.length > 0}
                        <span
                            class="shrink-0 rounded-full border border-amber-200 bg-amber-100 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-amber-700"
                        >
                            JIT goes BRRRR
                        </span>
                    {/if}
                </span>
                <span
                    class="inline-block transition-transform {isExpanded
                        ? 'rotate-90'
                        : ''}"
                    aria-hidden="true"
                >
                    ▸
                </span>
            </button>

            {#if isExpanded}
                <div
                    id={`bytecode-panel-${fnIndex}`}
                    transition:slide={{ duration: EXPAND_DURATION }}
                >
                    <div
                        class="flex flex-row w-full gap-2 max-h-[800px] pt-2"
                    >
                        <div
                            data-code-scroll
                            class="flex-1 min-w-0 overflow-hidden overflow-y-auto rounded-md p-1.5"
                        >
                            {#each byte_code as { op, ast_id, op_ptr }, i}
                                <p
                                    ondblclick={() => {
                                        setHighlight({
                                            astId: ast_id,
                                            source: "op_code",
                                            opCodePtr: op_ptr,
                                            fn,
                                        });
                                    }}
                                    data-byte-row
                                    data-fn={fn}
                                    data-ast-id={ast_id}
                                    data-op-ptr={op_ptr}
                                    class="code cursor-default select-none rounded border-l border-transparent px-2 py-0.5 whitespace-pre-wrap {Number(
                                        ast_id,
                                    )} {byteHighlightClasses[i]}"
                                >
                                    {op}
                                </p>
                            {/each}
                        </div>
                        {#if jit.length > 0}
                            <div
                                data-code-scroll
                                class="flex-1 min-w-0 overflow-hidden overflow-y-auto rounded-md p-1.5"
                            >
                                {#each jit as { inst, op_code_ptr, ast_id }, i}
                                    <p
                                        ondblclick={() => {
                                            setHighlight({
                                                astId: ast_id,
                                                source: "asm",
                                                opCodePtr: op_code_ptr,
                                                fn,
                                            });
                                        }}
                                        data-jit-row
                                        data-fn={fn}
                                        data-ast-id={ast_id}
                                        data-op-code-ptr={op_code_ptr}
                                        class="code cursor-default select-none rounded border-l border-transparent px-2 py-0.5 whitespace-pre-wrap ast-{Number(
                                            ast_id,
                                        )} op-{Number(
                                            op_code_ptr,
                                        )} {jitHighlightClasses[i]}"
                                    >
                                        {formatAssemblyInstruction(inst)}
                                    </p>
                                {/each}
                            </div>
                        {/if}
                    </div>
                </div>
            {/if}
        </div>
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
