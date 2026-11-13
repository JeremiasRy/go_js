<script lang="ts">
    import { tick, untrack } from "svelte";
    import { slide } from "svelte/transition";
    import type {
        DisassembledChunk,
        HighlightStatus,
        SetHighlightStatusParams,
    } from "../types";
    import OpcodeList from "./OpcodeList.svelte";

    type PropsType = {
        chunks: DisassembledChunk[];
        setHighlight: (opts: SetHighlightStatusParams) => void;
        highlight: HighlightStatus | null;
    };
    const opElements = new Map<number, HTMLElement>();
    const opByPtr = new Map<string, HTMLElement>();
    const { chunks, setHighlight, highlight }: PropsType = $props();

    const MAIN_FUNCTION = "PROGRAM_MAIN";
    const EXPAND_DURATION = 200;

    type CodeView = "bytecode" | "jit";

    let expandedChunks = $state<string[]>([]);
    let codeViews = $state<Record<string, CodeView>>({});

    function compareChunks(a: DisassembledChunk, b: DisassembledChunk) {
        if (a.name !== b.name) {
            return a.name < b.name ? -1 : 1;
        }
        if (a.id !== b.id) {
            return a.id < b.id ? -1 : 1;
        }
        return 0;
    }

    const sameChunks = (a: string[], b: string[]) => {
        return a.length === b.length && a.every((chunk) => b.includes(chunk));
    };

    function toggleChunk(chunkId: string) {
        expandedChunks = expandedChunks.includes(chunkId)
            ? expandedChunks.filter((id) => id !== chunkId)
            : [...expandedChunks, chunkId];
    }

    function codeViewFor(chunkId: string, hasJit: boolean): CodeView {
        if (!hasJit) {
            return "bytecode";
        }

        return codeViews[chunkId] ?? "jit";
    }

    function setCodeView(chunkId: string, view: CodeView) {
        codeViews = { ...codeViews, [chunkId]: view };
    }

    function codeViewButtonClass(isActive: boolean): string {
        return isActive
            ? "cursor-pointer rounded-md bg-white px-2.5 py-1 text-xs font-semibold text-slate-700 shadow-sm"
            : "cursor-pointer rounded-md px-2.5 py-1 text-xs font-medium text-slate-500 hover:text-slate-700";
    }

    const ptrKey = (chunkId: string, ptr: number) => `${chunkId}:${ptr}`;

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

    function rebuildElementMaps() {
        opElements.clear();
        opByPtr.clear();

        if (!codeScrollContainer) {
            return;
        }

        const expanded = new Set(expandedChunks);

        for (const row of codeScrollContainer.querySelectorAll<HTMLElement>(
            "[data-code-row]",
        )) {
            const chunkId = row.dataset.chunkId;
            if (chunkId === undefined || !expanded.has(chunkId)) {
                continue;
            }

            const astId = Number(row.dataset.astId);
            const key = ptrKey(chunkId, Number(row.dataset.opPtr));

            if (!opByPtr.has(key)) {
                opByPtr.set(key, row);
            }

            if (!opElements.has(astId)) {
                opElements.set(astId, row);
            }
        }
    }

    const chunkEntries = $derived(
        [...(chunks ?? [])]
            .filter((chunk) => !chunk.hidden)
            .sort(compareChunks),
    );

    const mainChunk = $derived(
        chunkEntries.find((chunk) => chunk.name === MAIN_FUNCTION) ??
            chunkEntries[0] ??
            null,
    );

    const astToChunks = $derived.by(() => {
        const map = new Map<number, Set<string>>();

        const add = (astId: number, chunkId: string) => {
            let chunks = map.get(astId);
            if (chunks === undefined) {
                chunks = new Set<string>();
                map.set(astId, chunks);
            }
            chunks.add(chunkId);
        };

        for (const chunk of chunkEntries) {
            for (const { ast_id } of chunk.byte_code) {
                add(ast_id, chunk.id);
            }

            for (const { ast_id } of chunk.jit) {
                add(ast_id, chunk.id);
            }
        }

        return map;
    });

    const affectedChunks = $derived.by(() => {
        if (highlight === null) {
            return [];
        }

        if (highlight.source !== "ast") {
            return chunkEntries.some((chunk) => chunk.id === highlight.chunkId)
                ? [highlight.chunkId]
                : mainChunk
                  ? [mainChunk.id]
                  : [];
        }

        const affected = new Set<string>();

        for (const astId of highlight.astIds) {
            for (const chunkId of astToChunks.get(astId) ?? []) {
                affected.add(chunkId);
            }
        }

        return affected.size > 0
            ? [...affected]
            : mainChunk
              ? [mainChunk.id]
              : [];
    });

    function scrollToHighlight(currentHighlight: HighlightStatus | null) {
        if (currentHighlight === null) {
            return;
        }

        rebuildElementMaps();

        let target: HTMLElement | null = null;

        if (currentHighlight.source === "ast") {
            target = earliestElement(
                currentHighlight.astIds
                    .map((n) => opElements.get(n))
                    .filter(
                        (element): element is HTMLElement =>
                            element !== undefined &&
                            element.offsetParent !== null,
                    ),
            );
        } else {
            target =
                opByPtr.get(
                    ptrKey(
                        currentHighlight.chunkId,
                        currentHighlight.opCodePtr,
                    ),
                ) ?? null;
        }

        if (target && !target.offsetParent) {
            target = null;
        }

        scrollTo(target);
    }

    $effect(() => {
        const currentHighlight = highlight;
        const affected = affectedChunks;
        const previous = untrack(() => expandedChunks);
        const expansionChanged = !sameChunks(previous, affected);

        if (expansionChanged) {
            expandedChunks = [...affected];
        }

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
            ? { chunkId: highlight.chunkId, ptr: highlight.opCodePtr }
            : null,
    );

    function buildHighlightClasses<T extends { ast_id: number }>({
        items,
        chunkId,
        getPtr,
    }: {
        items: T[];
        chunkId: string;
        getPtr: (item: T) => number;
    }) {
        return items.map((item) => {
            const byAst = highlightedAstIds.has(item.ast_id);
            const byPointer =
                highlightedPointer !== null &&
                highlightedPointer.chunkId === chunkId &&
                highlightedPointer.ptr === getPtr(item);

            return byAst || byPointer ? "highlight" : "";
        });
    }

    const getJitPtr = (item: { op_code_ptr: number }) => item.op_code_ptr;
</script>

<div
    bind:this={codeScrollContainer}
    class="flex flex-1 min-h-0 flex-col gap-2 overflow-y-auto"
>
    {#each chunkEntries as chunk, chunkIndex (chunk.id)}
        {@const isExpanded = expandedChunks.includes(chunk.id)}
        {@const jitHighlightClasses = buildHighlightClasses({
            items: chunk.jit,
            chunkId: chunk.id,
            getPtr: getJitPtr,
        })}
        {@const view = codeViewFor(chunk.id, chunk.jit.length > 0)}
        <div class="rounded-lg border border-slate-200 bg-white p-2">
            <button
                type="button"
                class="flex w-full cursor-pointer items-center justify-between gap-2 text-left text-sm font-semibold text-slate-600 hover:text-slate-800"
                aria-expanded={isExpanded}
                aria-controls={`code-panel-${chunkIndex}`}
                onclick={() => toggleChunk(chunk.id)}
            >
                <span class="flex min-w-0 items-center gap-2">
                    <span class="truncate">
                        {chunk.name === MAIN_FUNCTION ? "Main" : chunk.name}
                    </span>
                    {#if chunk.jit.length > 0}
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
                    id={`code-panel-${chunkIndex}`}
                    transition:slide={{ duration: EXPAND_DURATION }}
                >
                    {#if chunk.jit.length > 0}
                        <div
                            class="mt-2 flex items-center gap-1 rounded-lg bg-slate-100 p-1"
                            role="group"
                            aria-label={`${chunk.name} code view`}
                        >
                            <button
                                type="button"
                                aria-pressed={view === "bytecode"}
                                class={codeViewButtonClass(
                                    view === "bytecode",
                                )}
                                onclick={() =>
                                    setCodeView(chunk.id, "bytecode")}
                            >
                                Bytecode
                            </button>
                            <button
                                type="button"
                                aria-pressed={view === "jit"}
                                class={codeViewButtonClass(view === "jit")}
                                onclick={() => setCodeView(chunk.id, "jit")}
                            >
                                JIT assembly
                            </button>
                        </div>
                    {/if}

                    <div
                        class="mt-2 max-h-[800px] overflow-y-auto rounded-md p-1.5"
                    >
                        {#if view === "jit"}
                            {#each chunk.jit as { inst, op_code_ptr, ast_id }, i}
                                <p
                                    ondblclick={() => {
                                        setHighlight({
                                            astId: ast_id,
                                            source: "asm",
                                            opCodePtr: op_code_ptr,
                                            chunkId: chunk.id,
                                        });
                                    }}
                                    data-code-row
                                    data-chunk-id={chunk.id}
                                    data-ast-id={ast_id}
                                    data-op-ptr={op_code_ptr}
                                    class="code cursor-default select-none rounded border-l border-transparent px-2 py-0.5 whitespace-pre-wrap ast-{Number(
                                        ast_id,
                                    )} op-{Number(
                                        op_code_ptr,
                                    )} {jitHighlightClasses[i]}"
                                >
                                    {formatAssemblyInstruction(inst)}
                                </p>
                            {/each}
                        {:else}
                            <OpcodeList
                                chunk={chunk}
                                {highlight}
                                {setHighlight}
                            />
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
