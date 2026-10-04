<script lang="ts">
    import { basicSetup } from "codemirror";
    import { EditorView, Decoration } from "@codemirror/view";
    import {
        Compartment,
        EditorState,
        StateEffect,
        StateField,
    } from "@codemirror/state";
    import { javascript } from "@codemirror/lang-javascript";
    import { onMount } from "svelte";
    import fibo from "$lib/examples/fibonacci?raw";
    import type {
        AstNode,
        HighlightStatus,
        PageStatus,
        InterpretResult,
        SetHighlightStatusParams,
    } from "../types";
    import AstTree from "./AstTree.svelte";
    import { generateLookUp } from "$lib/util";
    import Code from "./Code.svelte";

    const readOnlyCompartment = new Compartment();
    const astElements = $state(new Map<number, HTMLElement>());
    const lookUp = $state(new Map<number, AstNode>());

    let editorContainer: HTMLElement;
    let view: EditorView;
    let jobId = $state<null | string>(null);
    let pageState = $state<PageStatus>("input");
    let interpretResult = $state<InterpretResult | null>(null);
    let showResults = $derived(
        pageState === "done" &&
            interpretResult !== null &&
            interpretResult.jobStatus === "Success" &&
            interpretResult.result !== null,
    );
    let highlight = $state.raw<HighlightStatus | null>(null);
    let timeout = $state.raw<number | null>(null);

    const isSameHighlight = (opts: SetHighlightStatusParams) => {
        if (highlight === null || opts === null) {
            return false;
        }

        if (highlight.source !== opts.source || highlight.astId !== opts.astId) {
            return false;
        }

        if (opts.source === "ast") {
            return true;
        }

        return highlight.fn === opts.fn && highlight.opCodePtr === opts.opCodePtr;
    };

    const setHighlight = (opts: SetHighlightStatusParams) => {
        if (opts === null || isSameHighlight(opts)) {
            highlight = null;
            return;
        }

        if (opts.source === "asm") {
            highlight = {
                source: "asm",
                astId: opts.astId,
                astIds: [opts.astId],
                opCodePtr: opts.opCodePtr,
                fn: opts.fn,
            };
            return;
        }

        highlight = {
            source: opts.source,
            astId: opts.astId,
            astIds: [opts.astId, ...(lookUp?.get(opts.astId)?.ast_train ?? [])],
            fn: opts.source === "op_code" ? opts.fn : "",
            opCodePtr: opts.source === "op_code" ? opts.opCodePtr : -1,
        };
    };

    const OUTPUT_TITLES: Record<PageStatus, string> = {
        input: "Results will appear here...",
        submitting: "Submitting...",
        polling: "Runnning code...",
        error: "Oops, something went seriously wrong :/",
        done: "no-op",
    };

    const addHighlight = StateEffect.define<{ from: number; to: number }>();
    const highlightMark = Decoration.mark({ class: "cm-highlight" });
    const highlightField = StateField.define({
        create() {
            return Decoration.none;
        },
        update(decorations, tr) {
            decorations = decorations.map(tr.changes);

            for (const { value } of tr.effects.filter((e) =>
                e.is(addHighlight),
            )) {
                const { from, to } = value;
                if (from === 0 && to === 0) {
                    decorations = Decoration.none;
                } else if (from < to) {
                    decorations = Decoration.set([
                        highlightMark.range(from, to),
                    ]);
                } else {
                    decorations = Decoration.none;
                }
            }
            return decorations;
        },
        provide: (field) => EditorView.decorations.from(field),
    });

    const highlightTheme = EditorView.baseTheme({
        ".cm-highlight": { backgroundColor: "yellow" },
    });

    onMount(() => {
        view = new EditorView({
            doc: fibo,
            parent: editorContainer,
            extensions: [
                basicSetup,
                javascript(),
                readOnlyCompartment.of([
                    EditorState.readOnly.of(false),
                    EditorView.editable.of(true),
                ]),
                highlightField,
                highlightTheme,
                EditorView.theme({
                    "&": {
                        height: "100%",
                        width: "100%",
                    },
                }),
            ],
        });
    });

    $effect(() => {
        if (jobId === null) {
            return;
        }

        const timeout = setInterval(async () => {
            try {
                const result = await fetch(
                    `http://localhost:8000/api/jobs/${jobId}`,
                );

                const json = await result.json();

                if (json.job_status === "Success") {
                    const interpretDetails = JSON.parse(atob(json.result));
                    jobId = null;
                    interpretResult = {
                        jobStatus: "Success",
                        result: interpretDetails,
                    };

                    generateLookUp({
                        node: interpretDetails.ast,
                        target: lookUp,
                    });
                    pageState = "done";
                    return;
                }

                if (json.job_status === "Failed") {
                    jobId = null;
                    interpretResult = json;
                    pageState = "done";
                    return;
                }
            } catch (e) {
                console.error(e);
                pageState = "error";
            }
        }, 500);

        return () => clearInterval(timeout);
    });

    $effect(() => {
        switch (pageState) {
            case "done":
            case "polling":
            case "submitting": {
                view.dispatch({
                    effects: readOnlyCompartment.reconfigure([
                        EditorState.readOnly.of(true),
                        EditorView.editable.of(false),
                    ]),
                });
                break;
            }
            case "input": {
                view.dispatch({
                    effects: readOnlyCompartment.reconfigure([
                        EditorState.readOnly.of(false),
                        EditorView.editable.of(true),
                    ]),
                });
                break;
            }
        }
    });

    $effect(() => {
        if (highlight === null) {
            view.dispatch({
                effects: addHighlight.of({ from: 0, to: 0 }),
            });
            return;
        }

        const { start: from, end: to } = lookUp?.get(highlight.astId) ?? {
            start: 0,
            end: 0,
        };

        view.dispatch({
            effects: addHighlight.of({ from, to }),
        });

        if (highlight.source === "op_code" || highlight.source === "asm") {
            timeout = setTimeout(() => {
                if (highlight === null) {
                    return;
                }
                const elements: HTMLElement[] = highlight.astIds
                    .filter((n) => astElements.has(n))
                    .map((n) => astElements.get(n)) as HTMLElement[];

                const firstElement = elements.reduce<HTMLElement | null>(
                    (earliest, current) => {
                        if (!earliest) return current;

                        const position =
                            earliest.compareDocumentPosition(current);
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
            }, 100);
        }

        return () => {
            if (timeout !== null) {
                clearTimeout(timeout);
                timeout = null;
            }
        };
    });

    async function submitCode() {
        const src = view.state.doc.toString();
        pageState = "submitting";

        try {
            const result = await fetch("http://localhost:8000/api/interpret", {
                method: "POST",
                body: JSON.stringify({ src }),
            });

            const { JobId } = (await result.json()) as { JobId: string };
            jobId = JobId;
            pageState = "polling";
        } catch (e) {
            pageState = "error";
        }
    }

    function resetPage() {
        pageState = "input";
        jobId = null;
        interpretResult = null;
    }

    function registerThySelf({ id, el }: { id: number; el: HTMLElement }) {
        astElements.set(id, el);
    }
</script>

<div
    class="w-full h-screen overflow-hidden flex flex-row gap-4 p-4 bg-slate-50"
>
    <div
        class="w-1/2 bg-white border border-slate-200 rounded-xl shadow-sm flex flex-col overflow-hidden"
    >
        <div
            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600 shrink-0"
        >
            Editor
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto relative">
            <div bind:this={editorContainer} class="absolute inset-0 p-2"></div>
        </div>

        <div
            class="p-4 border-t border-slate-100 flex flex-row gap-2 justify-between shrink-0"
        >
            <button
                onclick={submitCode}
                disabled={pageState !== "input"}
                class="bg-[#00ADD8] hover:bg-[#00758D] text-white px-4 py-2 rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
            >
                Submit
            </button>
            {#if pageState !== "input"}
                <button
                    onclick={resetPage}
                    class="bg-[#F3F4F6] hover:bg-[#E5E7EB] text-[#374151] px-4 py-2 rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                    Reset
                </button>
            {/if}
        </div>
    </div>

    <div
        class="w-1/2 bg-white border border-slate-200 rounded-xl shadow-sm flex flex-col"
    >
        <div
            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600 shrink-0"
        >
            Output
        </div>

        {#if showResults}
            <div class="flex flex-col w-full flex-1 min-h-0">
                <div class="p-6 text-slate-500 whitespace-pre-wrap shrink-0">
                    {interpretResult!.result?.output}
                </div>

                <div class="flex flex-row w-full p-2 gap-2 flex-1 min-h-0">
                    <div class="p-2 w-1/2 flex flex-col gap-2">
                        <div
                            class="bg-slate-100 px-4 py-2 border-slate-200 rounded-md text-sm font-semibold text-slate-600 shrink-0"
                        >
                            ECMATree
                        </div>
                        <AstTree
                            node={interpretResult!.result!.ast}
                            {highlight}
                            {setHighlight}
                            {registerThySelf}
                        />
                    </div>
                    <div class="p-2 w-1/2 flex flex-col gap-2">
                        <div
                            class="bg-slate-100 px-4 py-2 border-slate-200 rounded-md text-sm font-semibold text-slate-600 shrink-0"
                        >
                            Byte Code
                        </div>
                        <Code
                            debugInfo={interpretResult!.result!.debug_info}
                            {highlight}
                            {setHighlight}
                        />
                    </div>
                </div>
            </div>
        {:else}
            <div class="p-6 text-slate-500 flex-1 overflow-y-auto">
                {OUTPUT_TITLES[pageState]}
            </div>
        {/if}
    </div>
</div>
