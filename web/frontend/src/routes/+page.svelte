<script lang="ts">
    import { basicSetup } from "codemirror";
    import { EditorView } from "@codemirror/view";
    import { Compartment, EditorState } from "@codemirror/state";
    import { javascript } from "@codemirror/lang-javascript";
    import { onMount } from "svelte";
    import fibo from "$lib/examples/fibonacci?raw";

    type PageState = "input" | "submitting" | "polling" | "error" | "done";

    let editorContainer: HTMLElement;
    let view: EditorView;
    let readOnlyCompartment = new Compartment();
    let jobId = $state<null | string>(null);
    let pageState = $state<PageState>("input");
    let interpretResult = $state<unknown | null>(null);

    const OUTPUT_TITLES: Record<PageState, string> = {
        input: "Results will appear here...",
        submitting: "Submitting...",
        polling: "Running code...",
        error: "Oops, something went seriously wrong :/",
        done: "There we go!",
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
            const result = await fetch(
                `http://localhost:8000/api/jobs/${jobId}`,
            );

            const json = await result.json();

            if (json.job_status === "Success") {
                jobId = null;
                interpretResult = json;
                pageState = "done";
                return;
            }

            if (json.job_status === "Failed") {
                jobId = null;
                interpretResult = json;
                pageState = "done";
                return;
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
        const result = await fetch("http://localhost:8000/api/interpret", {
            method: "POST",
            body: JSON.stringify({ src }),
        });

        const { JobId } = (await result.json()) as { JobId: string };
        jobId = JobId;
        pageState = "polling";
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
        <div class="p-4 border-t border-slate-100">
            <button
                onclick={submitCode}
                disabled={pageState !== "input"}
                class="bg-[#00ADD8] hover:bg-[#00758D] text-white px-4 py-2 rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer align-end"
            >
                Submit
            </button>
        </div>
    </div>

    <div
        class="w-1/2 h-full bg-white border border-slate-200 rounded-xl shadow-sm"
    >
        <div
            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600"
        >
            Output
        </div>
        <div class="p-6 text-slate-500">{OUTPUT_TITLES[pageState]}</div>
        <div class="p-6">
            {JSON.stringify(interpretResult, undefined, 2)}
        </div>
    </div>
</div>
