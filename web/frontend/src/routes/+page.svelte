<script lang="ts">
    import { basicSetup } from "codemirror";
    import { EditorView } from "@codemirror/view";
    import { Compartment, EditorState } from "@codemirror/state";
    import { javascript } from "@codemirror/lang-javascript";
    import { onMount } from "svelte";
    import fibo from "$lib/examples/fibonacci?raw";

    type PageStatus = "input" | "submitting" | "polling" | "error" | "done";
    type JobStatus = "Success" | "Failed" | "Pending" | "Processing";
    interface AstNode {
        id: number;
        start: number;
        end: number;
        type: string;
    }
    type InterpretDetails = {
        output: string;
        code: { ast_id: number; op: string };
        ast: AstNode;
    };
    type InterpretResult = {
        jobStatus: JobStatus;
        result: InterpretDetails | null;
    };
    const readOnlyCompartment = new Compartment();

    let editorContainer: HTMLElement;
    let view: EditorView;
    let jobId = $state<null | string>(null);
    let pageState = $state<PageStatus>("input");
    let interpretResult = $state<InterpretResult | null>(null);

    const OUTPUT_TITLES: Record<PageStatus, string> = {
        input: "Results will appear here...",
        submitting: "Submitting...",
        polling: "Runnning code...",
        error: "Oops, something went seriously wrong :/",
        done: "no-op",
    };

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
                    jobId = null;
                    interpretResult = {
                        jobStatus: "Success",
                        result: JSON.parse(atob(json.result)),
                    };
                    pageState = "done";
                    return;
                }

                if (json.job_status === "Failed") {
                    jobId = null;
                    interpretResult = json;
                    pageState = "done";
                    return;
                }
            } catch {
                console.log("errored");
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
</script>

<div class="w-full h-full flex flex-row gap-4 p-4">
    <div
        class="w-1/2 h-full bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden flex flex-col"
    >
        <div
            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600"
        >
            Editor
        </div>
        <div bind:this={editorContainer} class="h-full w-full p-2"></div>
        <div
            class="p-4 border-t border-slate-100 flex flex-row gap-2 justify-between"
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
        class="w-1/2 h-fit bg-white border border-slate-200 rounded-xl shadow-sm"
    >
        <div
            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600"
        >
            Output
        </div>

        {#if pageState === "done" && interpretResult !== null && interpretResult.jobStatus === "Success"}
            <div class="flex flex-col">
                <div class="p-6 text-slate-500 whitespace-pre-wrap">
                    {interpretResult.result?.output}
                </div>
                <div class="flex flex-row w-full p-4 gap-2">
                    <div
                        class="w-1/2 h-full bg-white border border-slate-200 rounded-xl shadow-sm"
                    >
                        <div
                            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600"
                        >
                            ECMATree
                        </div>
                        <p>
                            {JSON.stringify(
                                interpretResult.result?.ast,
                                undefined,
                                2,
                            )}
                        </p>
                    </div>
                    <div
                        class="w-1/2 h-full bg-white border border-slate-200 rounded-xl shadow-sm"
                    >
                        <div
                            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600"
                        >
                            Byte Code
                        </div>
                        <p>
                            {JSON.stringify(
                                interpretResult.result?.code,
                                undefined,
                                2,
                            )}
                        </p>
                    </div>
                </div>
            </div>
        {:else}
            <div class="p-6 text-slate-500">
                {OUTPUT_TITLES[pageState]}
            </div>
        {/if}
    </div>
</div>
