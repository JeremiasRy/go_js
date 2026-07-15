<script lang="ts">
    import { basicSetup } from "codemirror";
    import { EditorView, Decoration } from "@codemirror/view";
    import { Compartment, EditorState, StateEffect, StateField } from "@codemirror/state";
    import { javascript } from "@codemirror/lang-javascript";
    import { onMount } from "svelte";
    import fibo from "$lib/examples/fibonacci?raw";
    import type { AstNode, HighlightType } from "../types";
    import AstTree from "./AstTree.svelte";
    import ByteCode from "./ByteCode.svelte";

    type PageStatus = "input" | "submitting" | "polling" | "error" | "done";
    type JobStatus = "Success" | "Failed" | "Pending" | "Processing";
    type FunctionName = string;

    type InterpretDetails = {
        output: string;
        code: Record<FunctionName, { ast_id: number; op: string }[]>;
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
    let showResults = $derived(pageState === "done" && interpretResult !== null && interpretResult.jobStatus === "Success" && interpretResult.result !== null)
    let highlight = $state.raw<HighlightType>({ast_ids: [], from:0, to:0})

    const OUTPUT_TITLES: Record<PageStatus, string> = {
        input: "Results will appear here...",
        submitting: "Submitting...",
        polling: "Runnning code...",
        error: "Oops, something went seriously wrong :/",
        done: "no-op",
    };

    const setHighlight = (h: HighlightType) => {
        highlight = h
    } 

    const addHighlight = StateEffect.define<{from: number, to: number}>();
    const highlightMark = Decoration.mark({ class: "cm-highlight" });
    const highlightField = StateField.define({
      create() {
        return Decoration.none;
      },
      update(decorations, tr) {
        decorations = decorations.map(tr.changes);

        for (const {value} of tr.effects.filter((e) => e.is(addHighlight))) {
            const {from, to} = value
            if (from === 0 && to === 0) {
                decorations = Decoration.none;
            } else {
                decorations = Decoration.set([
                    highlightMark.range(from, to)
                ]);
            }
        }
        return decorations;
      },
      provide: field => EditorView.decorations.from(field)
    });

    const highlightTheme = EditorView.baseTheme({
      ".cm-highlight": { backgroundColor: "yellow" }
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
                
                /*
                const result = await fetch(
                    `http://localhost:8000/api/jobs/${jobId}`,
                    );*/
                    
                    const json = {job_status: "Success"};
                    
                    if (json.job_status === "Success") {
                        jobId = null;
                        interpretResult = {
                            jobStatus: "Success",
                            result: JSON.parse(atob(`eyJvdXRwdXQiOiJSZXN1bHQ6IDVcbkV4ZWN1dGlvbiBUaW1lOiAwbXNcbiIsImNvZGUiOnsiSU5URVJOQUxfU0VUVVAiOlt7Im9wIjoiMDAwMCB8IE9QX0NPTlNUQU5UIFxuMDAwNCB8IDAgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDAwOCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDAxMiB8IE9QX0NPTlNUQU5UIFxuMDAxNiB8IDEgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDAyMCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDAyNCB8IE9QX0NPTlNUQU5UIFxuMDAyOCB8IDIgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDAzMiB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDAzNiB8IE9QX0NPTlNUQU5UIFxuMDA0MCB8IDMgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA0NCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA0OCB8IE9QX0NPTlNUQU5UIFxuMDA1MiB8IDQgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA1NiB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA2MCB8IE9QX0NPTlNUQU5UIFxuMDA2NCB8IDUgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA2OCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA3MiB8IE9QX0NPTlNUQU5UIFxuMDA3NiB8IDYgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA4MCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA4NCB8IE9QX0NPTlNUQU5UIFxuMDA4OCB8IDcgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA5MiB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDA5NiB8IE9QX0NPTlNUQU5UIFxuMDEwMCB8IDggXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDEwNCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDEwOCB8IE9QX0NPTlNUQU5UIFxuMDExMiB8IDkgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDExNiB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDEyMCB8IE9QX0NPTlNUQU5UIFxuMDEyNCB8IDEwIFxuIiwiYXN0X2lkIjotMn0seyJvcCI6IjAxMjggfCBPUF9ERUZJTkVfR0xPQkFMIFxuIiwiYXN0X2lkIjotMn0seyJvcCI6IjAxMzIgfCBPUF9DT05TVEFOVCBcbjAxMzYgfCAxMSBcbiIsImFzdF9pZCI6LTJ9LHsib3AiOiIwMTQwIHwgT1BfQ0FMTCBcbjAxNDQgfCAwIFxuMDE0OCB8IHNwcmVhZDogZmFsc2UgXG4iLCJhc3RfaWQiOi0yfSx7Im9wIjoiMDE1MiB8IE9QX1JFVFVSTiBcbiIsImFzdF9pZCI6LTJ9XSwiUFJPR1JBTV9NQUlOIjpbeyJvcCI6IjAwMDAgfCBPUF9DT05TVEFOVCBcbjAwMDQgfCAwIFxuIiwiYXN0X2lkIjowfSx7Im9wIjoiMDAwOCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOjB9LHsib3AiOiIwMDEyIHwgT1BfR0VUX0dMT0JBTCBcbjAwMTYgfCA2IFxuIiwiYXN0X2lkIjoyN30seyJvcCI6IjAwMjAgfCBPUF9DT05TVEFOVCBcbjAwMjQgfCAxIFxuIiwiYXN0X2lkIjoyOH0seyJvcCI6IjAwMjggfCBPUF9HRVRfT0JKRUNUX01FTUJFUiBcbiIsImFzdF9pZCI6Mjh9LHsib3AiOiIwMDMyIHwgT1BfQ0FMTCBcbjAwMzYgfCAwIFxuMDA0MCB8IHNwcmVhZDogZmFsc2UgXG4iLCJhc3RfaWQiOjMwfSx7Im9wIjoiMDA0NCB8IE9QX0RFRklORV9HTE9CQUwgXG4iLCJhc3RfaWQiOjI1fSx7Im9wIjoiMDA0OCB8IE9QX0NPTlNUQU5UIFxuMDA1MiB8IDIgXG4iLCJhc3RfaWQiOjM1fSx7Im9wIjoiMDA1NiB8IE9QX0dFVF9HTE9CQUwgXG4wMDYwIHwgMTEgXG4iLCJhc3RfaWQiOjM0fSx7Im9wIjoiMDA2NCB8IE9QX0NBTEwgXG4wMDY4IHwgMSBcbjAwNzIgfCBzcHJlYWQ6IGZhbHNlIFxuIiwiYXN0X2lkIjozNn0seyJvcCI6IjAwNzYgfCBPUF9ERUZJTkVfR0xPQkFMIFxuIiwiYXN0X2lkIjozMn0seyJvcCI6IjAwODAgfCBPUF9HRVRfR0xPQkFMIFxuMDA4NCB8IDYgXG4iLCJhc3RfaWQiOjQwfSx7Im9wIjoiMDA4OCB8IE9QX0NPTlNUQU5UIFxuMDA5MiB8IDMgXG4iLCJhc3RfaWQiOjQxfSx7Im9wIjoiMDA5NiB8IE9QX0dFVF9PQkpFQ1RfTUVNQkVSIFxuIiwiYXN0X2lkIjo0MX0seyJvcCI6IjAxMDAgfCBPUF9DQUxMIFxuMDEwNCB8IDAgXG4wMTA4IHwgc3ByZWFkOiBmYWxzZSBcbiIsImFzdF9pZCI6NDN9LHsib3AiOiIwMTEyIHwgT1BfREVGSU5FX0dMT0JBTCBcbiIsImFzdF9pZCI6Mzh9LHsib3AiOiIwMTE2IHwgT1BfQ09OU1RBTlQgXG4wMTIwIHwgNCBcbiIsImFzdF9pZCI6NDh9LHsib3AiOiIwMTI0IHwgT1BfR0VUX0dMT0JBTCBcbjAxMjggfCAxMyBcbiIsImFzdF9pZCI6NTB9LHsib3AiOiIwMTMyIHwgT1BfQUREIFxuIiwiYXN0X2lkIjo0OH0seyJvcCI6IjAxMzYgfCBPUF9DT05TVEFOVCBcbjAxNDAgfCA1IFxuIiwiYXN0X2lkIjo0OH0seyJvcCI6IjAxNDQgfCBPUF9BREQgXG4iLCJhc3RfaWQiOjQ4fSx7Im9wIjoiMDE0OCB8IE9QX0dFVF9HTE9CQUwgXG4wMTUyIHwgMSBcbiIsImFzdF9pZCI6NDV9LHsib3AiOiIwMTU2IHwgT1BfQ09OU1RBTlQgXG4wMTYwIHwgNiBcbiIsImFzdF9pZCI6NDZ9LHsib3AiOiIwMTY0IHwgT1BfR0VUX09CSkVDVF9NRU1CRVIgXG4iLCJhc3RfaWQiOjQ2fSx7Im9wIjoiMDE2OCB8IE9QX0NBTEwgXG4wMTcyIHwgMSBcbjAxNzYgfCBzcHJlYWQ6IGZhbHNlIFxuIiwiYXN0X2lkIjo1Mn0seyJvcCI6IjAxODAgfCBPUF9QT1AgXG4iLCJhc3RfaWQiOjUyfSx7Im9wIjoiMDE4NCB8IE9QX0NPTlNUQU5UIFxuMDE4OCB8IDcgXG4iLCJhc3RfaWQiOjU3fSx7Im9wIjoiMDE5MiB8IE9QX0dFVF9HTE9CQUwgXG4wMTk2IHwgMTQgXG4iLCJhc3RfaWQiOjU5fSx7Im9wIjoiMDIwMCB8IE9QX0dFVF9HTE9CQUwgXG4wMjA0IHwgMTIgXG4iLCJhc3RfaWQiOjYwfSx7Im9wIjoiMDIwOCB8IE9QX1NVQlRSQUNUIFxuIiwiYXN0X2lkIjo2MX0seyJvcCI6IjAyMTIgfCBPUF9BREQgXG4iLCJhc3RfaWQiOjU3fSx7Im9wIjoiMDIxNiB8IE9QX0NPTlNUQU5UIFxuMDIyMCB8IDggXG4iLCJhc3RfaWQiOjU3fSx7Im9wIjoiMDIyNCB8IE9QX0FERCBcbiIsImFzdF9pZCI6NTd9LHsib3AiOiIwMjI4IHwgT1BfR0VUX0dMT0JBTCBcbjAyMzIgfCAxIFxuIiwiYXN0X2lkIjo1NH0seyJvcCI6IjAyMzYgfCBPUF9DT05TVEFOVCBcbjAyNDAgfCA5IFxuIiwiYXN0X2lkIjo1NX0seyJvcCI6IjAyNDQgfCBPUF9HRVRfT0JKRUNUX01FTUJFUiBcbiIsImFzdF9pZCI6NTV9LHsib3AiOiIwMjQ4IHwgT1BfQ0FMTCBcbjAyNTIgfCAxIFxuMDI1NiB8IHNwcmVhZDogZmFsc2UgXG4iLCJhc3RfaWQiOjYzfSx7Im9wIjoiMDI2MCB8IE9QX1BPUCBcbiIsImFzdF9pZCI6NjN9LHsib3AiOiIwMjY0IHwgT1BfUkVUVVJOIFxuIiwiYXN0X2lkIjowfV0sImZpYiI6W3sib3AiOiIwMDAwIHwgT1BfR0VUX0xPQ0FMIFxuMDAwNCB8IDAgXG4iLCJhc3RfaWQiOjZ9LHsib3AiOiIwMDA4IHwgT1BfQ09OU1RBTlQgXG4wMDEyIHwgMCBcbiIsImFzdF9pZCI6N30seyJvcCI6IjAwMTYgfCBPUF9MRVNTX1RIQU5fRVFVQUwgXG4iLCJhc3RfaWQiOjh9LHsib3AiOiIwMDI0IHwgT1BfSlVNUF9JRl9GQUxTRSBcbjAwMzYgfCA1MlxuIiwiYXN0X2lkIjo1fSx7Im9wIjoiMDA0MCB8IE9QX0dFVF9MT0NBTCBcbjAwNDQgfCAwIFxuIiwiYXN0X2lkIjoxMX0seyJvcCI6IjAwNDggfCBPUF9SRVRVUk4gXG4iLCJhc3RfaWQiOjEwfSx7Im9wIjoiMDA1MiB8IE9QX0dFVF9MT0NBTCBcbjAwNTYgfCAwIFxuIiwiYXN0X2lkIjoxNH0seyJvcCI6IjAwNjAgfCBPUF9DT05TVEFOVCBcbjAwNjQgfCAxIFxuIiwiYXN0X2lkIjoxNX0seyJvcCI6IjAwNjggfCBPUF9TVUJUUkFDVCBcbiIsImFzdF9pZCI6MTZ9LHsib3AiOiIwMDcyIHwgT1BfR0VUX0dMT0JBTCBcbjAwNzYgfCAxMSBcbiIsImFzdF9pZCI6MTN9LHsib3AiOiIwMDgwIHwgT1BfQ0FMTCBcbjAwODQgfCAxIFxuMDA4OCB8IHNwcmVhZDogZmFsc2UgXG4iLCJhc3RfaWQiOjE3fSx7Im9wIjoiMDA5MiB8IE9QX0dFVF9MT0NBTCBcbjAwOTYgfCAwIFxuIiwiYXN0X2lkIjoxOX0seyJvcCI6IjAxMDAgfCBPUF9DT05TVEFOVCBcbjAxMDQgfCAyIFxuIiwiYXN0X2lkIjoyMH0seyJvcCI6IjAxMDggfCBPUF9TVUJUUkFDVCBcbiIsImFzdF9pZCI6MjF9LHsib3AiOiIwMTEyIHwgT1BfR0VUX0dMT0JBTCBcbjAxMTYgfCAxMSBcbiIsImFzdF9pZCI6MTh9LHsib3AiOiIwMTIwIHwgT1BfQ0FMTCBcbjAxMjQgfCAxIFxuMDEyOCB8IHNwcmVhZDogZmFsc2UgXG4iLCJhc3RfaWQiOjIyfSx7Im9wIjoiMDEzMiB8IE9QX0FERCBcbiIsImFzdF9pZCI6MjN9LHsib3AiOiIwMTM2IHwgT1BfUkVUVVJOIFxuIiwiYXN0X2lkIjoxMn1dfSwiYXN0Ijp7ImlkIjowLCJhc3RfdHJhaW4iOlsxLDUsOCw2LDcsOSwxMCwxMSwxMiwyMywxNywxNiwxNCwxNSwxMywyMiwyMSwxOSwyMCwxOCwyNCwyNSwzMCwyOCwyNywzMSwzMiwzNiwzNSwzNCwzNywzOCw0Myw0MSw0MCw0NCw1Miw0OCw1MCw0Niw0NSw1Myw2Myw1Nyw2MSw1OSw2MCw1NSw1NF0sInN0YXJ0IjowLCJlbmQiOjI2MSwidHlwZSI6IlByb2dyYW0iLCJib2R5IjpbeyJpZCI6MSwiYXN0X3RyYWluIjpbNSw4LDYsNyw5LDEwLDExLDEyLDIzLDE3LDE2LDE0LDE1LDEzLDIyLDIxLDE5LDIwLDE4XSwic3RhcnQiOjAsImVuZCI6ODcsInR5cGUiOiJGdW5jdGlvbkRlY2xhcmF0aW9uIiwiYm9keU5vZGUiOnsiaWQiOjQsImFzdF90cmFpbiI6bnVsbCwic3RhcnQiOjE2LCJlbmQiOjg3LCJ0eXBlIjoiQmxvY2tTdGF0ZW1lbnQiLCJib2R5IjpbeyJpZCI6NSwiYXN0X3RyYWluIjpbOCw2LDcsOSwxMCwxMV0sInN0YXJ0IjoyMCwiZW5kIjo1MSwidHlwZSI6IklmU3RhdGVtZW50IiwidGVzdCI6eyJpZCI6OCwiYXN0X3RyYWluIjpbNiw3XSwic3RhcnQiOjI0LCJlbmQiOjMwLCJ0eXBlIjoiQmluYXJ5RXhwcmVzc2lvbiIsImJpbmFyeU9wZXJhdG9yIjoiXHUwMDNjPSIsImxlZnQiOnsiaWQiOjYsImFzdF90cmFpbiI6W10sInN0YXJ0IjoyNCwiZW5kIjoyNSwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoibiJ9LCJyaWdodCI6eyJpZCI6NywiYXN0X3RyYWluIjpbXSwic3RhcnQiOjI5LCJlbmQiOjMwLCJ0eXBlIjoiTGl0ZXJhbCIsInJhdyI6IjEiLCJ2YWx1ZSI6MX19LCJjb25zZXF1ZW50Ijp7ImlkIjo5LCJhc3RfdHJhaW4iOlsxMCwxMV0sInN0YXJ0IjozMiwiZW5kIjo1MSwidHlwZSI6IkJsb2NrU3RhdGVtZW50IiwiYm9keSI6W3siaWQiOjEwLCJhc3RfdHJhaW4iOlsxMV0sInN0YXJ0IjozOCwiZW5kIjo0NywidHlwZSI6IlJldHVyblN0YXRlbWVudCIsImFyZ3VtZW50Ijp7ImlkIjoxMSwiYXN0X3RyYWluIjpbXSwic3RhcnQiOjQ1LCJlbmQiOjQ2LCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJuIn19XX19LHsiaWQiOjEyLCJhc3RfdHJhaW4iOlsyMywxNywxNiwxNCwxNSwxMywyMiwyMSwxOSwyMCwxOF0sInN0YXJ0Ijo1NCwiZW5kIjo4NSwidHlwZSI6IlJldHVyblN0YXRlbWVudCIsImFyZ3VtZW50Ijp7ImlkIjoyMywiYXN0X3RyYWluIjpbMTcsMTYsMTQsMTUsMTMsMjIsMjEsMTksMjAsMThdLCJzdGFydCI6NjEsImVuZCI6ODQsInR5cGUiOiJCaW5hcnlFeHByZXNzaW9uIiwiYmluYXJ5T3BlcmF0b3IiOiIrIiwibGVmdCI6eyJpZCI6MTcsImFzdF90cmFpbiI6WzE2LDE0LDE1LDEzXSwic3RhcnQiOjYxLCJlbmQiOjcxLCJ0eXBlIjoiQ2FsbEV4cHJlc3Npb24iLCJjYWxsZWUiOnsiaWQiOjEzLCJhc3RfdHJhaW4iOltdLCJzdGFydCI6NjEsImVuZCI6NjQsInR5cGUiOiJJZGVudGlmaWVyIiwibmFtZSI6ImZpYiJ9LCJhcmd1bWVudHMiOlt7ImlkIjoxNiwiYXN0X3RyYWluIjpbMTQsMTVdLCJzdGFydCI6NjUsImVuZCI6NzAsInR5cGUiOiJCaW5hcnlFeHByZXNzaW9uIiwiYmluYXJ5T3BlcmF0b3IiOiItIiwibGVmdCI6eyJpZCI6MTQsImFzdF90cmFpbiI6W10sInN0YXJ0Ijo2NSwiZW5kIjo2NiwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoibiJ9LCJyaWdodCI6eyJpZCI6MTUsImFzdF90cmFpbiI6W10sInN0YXJ0Ijo2OSwiZW5kIjo3MCwidHlwZSI6IkxpdGVyYWwiLCJyYXciOiIxIiwidmFsdWUiOjF9fV19LCJyaWdodCI6eyJpZCI6MjIsImFzdF90cmFpbiI6WzIxLDE5LDIwLDE4XSwic3RhcnQiOjc0LCJlbmQiOjg0LCJ0eXBlIjoiQ2FsbEV4cHJlc3Npb24iLCJjYWxsZWUiOnsiaWQiOjE4LCJhc3RfdHJhaW4iOltdLCJzdGFydCI6NzQsImVuZCI6NzcsInR5cGUiOiJJZGVudGlmaWVyIiwibmFtZSI6ImZpYiJ9LCJhcmd1bWVudHMiOlt7ImlkIjoyMSwiYXN0X3RyYWluIjpbMTksMjBdLCJzdGFydCI6NzgsImVuZCI6ODMsInR5cGUiOiJCaW5hcnlFeHByZXNzaW9uIiwiYmluYXJ5T3BlcmF0b3IiOiItIiwibGVmdCI6eyJpZCI6MTksImFzdF90cmFpbiI6W10sInN0YXJ0Ijo3OCwiZW5kIjo3OSwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoibiJ9LCJyaWdodCI6eyJpZCI6MjAsImFzdF90cmFpbiI6W10sInN0YXJ0Ijo4MiwiZW5kIjo4MywidHlwZSI6IkxpdGVyYWwiLCJyYXciOiIyIiwidmFsdWUiOjJ9fV19fX1dfSwiaWRlbnRpZmllciI6eyJpZCI6MiwiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6OSwiZW5kIjoxMiwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoiZmliIn0sInBhcmFtcyI6W3siaWQiOjMsImFzdF90cmFpbiI6bnVsbCwic3RhcnQiOjEzLCJlbmQiOjE0LCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJuIn1dfSx7ImlkIjoyNCwiYXN0X3RyYWluIjpbMjUsMzAsMjgsMjddLCJzdGFydCI6ODksImVuZCI6MTE4LCJ0eXBlIjoiVmFyaWFibGVEZWNsYXJhdGlvbiIsImRlY2xhcmF0aW9ucyI6W3siaWQiOjI1LCJhc3RfdHJhaW4iOlszMCwyOCwyN10sInN0YXJ0Ijo5NSwiZW5kIjoxMTcsInR5cGUiOiJWYXJpYWJsZURlY2xhcmF0b3IiLCJpZGVudGlmaWVyIjp7ImlkIjoyNiwiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6OTUsImVuZCI6MTA0LCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJzdGFydFRpbWUifSwiaW5pdGlhbGl6ZXIiOnsiaWQiOjMwLCJhc3RfdHJhaW4iOlsyOCwyN10sInN0YXJ0IjoxMDcsImVuZCI6MTE3LCJ0eXBlIjoiQ2FsbEV4cHJlc3Npb24iLCJjYWxsZWUiOnsiaWQiOjI4LCJhc3RfdHJhaW4iOlsyN10sInN0YXJ0IjoxMDcsImVuZCI6MTE1LCJ0eXBlIjoiTWVtYmVyRXhwcmVzc2lvbiIsIm9iamVjdCI6eyJpZCI6MjcsImFzdF90cmFpbiI6W10sInN0YXJ0IjoxMDcsImVuZCI6MTExLCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJEYXRlIn0sInByb3BlcnR5Ijp7ImlkIjoyOSwiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6MTEyLCJlbmQiOjExNSwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoibm93In19fX1dLCJraW5kIjoiY29uc3QifSx7ImlkIjozMSwiYXN0X3RyYWluIjpbMzIsMzYsMzUsMzRdLCJzdGFydCI6MTE5LCJlbmQiOjE0MSwidHlwZSI6IlZhcmlhYmxlRGVjbGFyYXRpb24iLCJkZWNsYXJhdGlvbnMiOlt7ImlkIjozMiwiYXN0X3RyYWluIjpbMzYsMzUsMzRdLCJzdGFydCI6MTI1LCJlbmQiOjE0MCwidHlwZSI6IlZhcmlhYmxlRGVjbGFyYXRvciIsImlkZW50aWZpZXIiOnsiaWQiOjMzLCJhc3RfdHJhaW4iOm51bGwsInN0YXJ0IjoxMjUsImVuZCI6MTMxLCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJyZXN1bHQifSwiaW5pdGlhbGl6ZXIiOnsiaWQiOjM2LCJhc3RfdHJhaW4iOlszNSwzNF0sInN0YXJ0IjoxMzQsImVuZCI6MTQwLCJ0eXBlIjoiQ2FsbEV4cHJlc3Npb24iLCJjYWxsZWUiOnsiaWQiOjM0LCJhc3RfdHJhaW4iOltdLCJzdGFydCI6MTM0LCJlbmQiOjEzNywidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoiZmliIn0sImFyZ3VtZW50cyI6W3siaWQiOjM1LCJhc3RfdHJhaW4iOltdLCJzdGFydCI6MTM4LCJlbmQiOjEzOSwidHlwZSI6IkxpdGVyYWwiLCJyYXciOiI1IiwidmFsdWUiOjV9XX19XSwia2luZCI6ImNvbnN0In0seyJpZCI6MzcsImFzdF90cmFpbiI6WzM4LDQzLDQxLDQwXSwic3RhcnQiOjE0MiwiZW5kIjoxNjksInR5cGUiOiJWYXJpYWJsZURlY2xhcmF0aW9uIiwiZGVjbGFyYXRpb25zIjpbeyJpZCI6MzgsImFzdF90cmFpbiI6WzQzLDQxLDQwXSwic3RhcnQiOjE0OCwiZW5kIjoxNjgsInR5cGUiOiJWYXJpYWJsZURlY2xhcmF0b3IiLCJpZGVudGlmaWVyIjp7ImlkIjozOSwiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6MTQ4LCJlbmQiOjE1NSwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoiZW5kVGltZSJ9LCJpbml0aWFsaXplciI6eyJpZCI6NDMsImFzdF90cmFpbiI6WzQxLDQwXSwic3RhcnQiOjE1OCwiZW5kIjoxNjgsInR5cGUiOiJDYWxsRXhwcmVzc2lvbiIsImNhbGxlZSI6eyJpZCI6NDEsImFzdF90cmFpbiI6WzQwXSwic3RhcnQiOjE1OCwiZW5kIjoxNjYsInR5cGUiOiJNZW1iZXJFeHByZXNzaW9uIiwib2JqZWN0Ijp7ImlkIjo0MCwiYXN0X3RyYWluIjpbXSwic3RhcnQiOjE1OCwiZW5kIjoxNjIsInR5cGUiOiJJZGVudGlmaWVyIiwibmFtZSI6IkRhdGUifSwicHJvcGVydHkiOnsiaWQiOjQyLCJhc3RfdHJhaW4iOm51bGwsInN0YXJ0IjoxNjMsImVuZCI6MTY2LCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJub3cifX19fV0sImtpbmQiOiJjb25zdCJ9LHsiaWQiOjQ0LCJhc3RfdHJhaW4iOls1Miw0OCw1MCw0Niw0NV0sInN0YXJ0IjoxNzEsImVuZCI6MjA0LCJ0eXBlIjoiRXhwcmVzc2lvblN0YXRlbWVudCIsImV4cHJlc3Npb24iOnsiaWQiOjUyLCJhc3RfdHJhaW4iOls0OCw1MCw0Niw0NV0sInN0YXJ0IjoxNzEsImVuZCI6MjAzLCJ0eXBlIjoiQ2FsbEV4cHJlc3Npb24iLCJjYWxsZWUiOnsiaWQiOjQ2LCJhc3RfdHJhaW4iOls0NV0sInN0YXJ0IjoxNzEsImVuZCI6MTgyLCJ0eXBlIjoiTWVtYmVyRXhwcmVzc2lvbiIsIm9iamVjdCI6eyJpZCI6NDUsImFzdF90cmFpbiI6W10sInN0YXJ0IjoxNzEsImVuZCI6MTc4LCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJjb25zb2xlIn0sInByb3BlcnR5Ijp7ImlkIjo0NywiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6MTc5LCJlbmQiOjE4MiwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoibG9nIn19LCJhcmd1bWVudHMiOlt7ImlkIjo0OCwiYXN0X3RyYWluIjpbNTBdLCJzdGFydCI6MTgzLCJlbmQiOjIwMiwidHlwZSI6IlRlbXBsYXRlTGl0ZXJhbCIsImV4cHJlc3Npb25zIjpbeyJpZCI6NTAsImFzdF90cmFpbiI6W10sInN0YXJ0IjoxOTQsImVuZCI6MjAwLCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJyZXN1bHQifV0sInF1YXNpcyI6W3siaWQiOjQ5LCJhc3RfdHJhaW4iOm51bGwsInN0YXJ0IjoxODQsImVuZCI6MTkyLCJ0eXBlIjoiVGVtcGxhdGVFbGVtZW50IiwidmFsdWUiOnsiUmF3IjoiUmVzdWx0OiAiLCJDb29rZWQiOiJSZXN1bHQ6ICJ9fSx7ImlkIjo1MSwiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6MjAxLCJlbmQiOjIwMSwidHlwZSI6IlRlbXBsYXRlRWxlbWVudCIsInRhaWwiOnRydWUsInZhbHVlIjp7IlJhdyI6IiIsIkNvb2tlZCI6IiJ9fV19XX19LHsiaWQiOjUzLCJhc3RfdHJhaW4iOls2Myw1Nyw2MSw1OSw2MCw1NSw1NF0sInN0YXJ0IjoyMDUsImVuZCI6MjYxLCJ0eXBlIjoiRXhwcmVzc2lvblN0YXRlbWVudCIsImV4cHJlc3Npb24iOnsiaWQiOjYzLCJhc3RfdHJhaW4iOls1Nyw2MSw1OSw2MCw1NSw1NF0sInN0YXJ0IjoyMDUsImVuZCI6MjYwLCJ0eXBlIjoiQ2FsbEV4cHJlc3Npb24iLCJjYWxsZWUiOnsiaWQiOjU1LCJhc3RfdHJhaW4iOls1NF0sInN0YXJ0IjoyMDUsImVuZCI6MjE2LCJ0eXBlIjoiTWVtYmVyRXhwcmVzc2lvbiIsIm9iamVjdCI6eyJpZCI6NTQsImFzdF90cmFpbiI6W10sInN0YXJ0IjoyMDUsImVuZCI6MjEyLCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJjb25zb2xlIn0sInByb3BlcnR5Ijp7ImlkIjo1NiwiYXN0X3RyYWluIjpudWxsLCJzdGFydCI6MjEzLCJlbmQiOjIxNiwidHlwZSI6IklkZW50aWZpZXIiLCJuYW1lIjoibG9nIn19LCJhcmd1bWVudHMiOlt7ImlkIjo1NywiYXN0X3RyYWluIjpbNjEsNTksNjBdLCJzdGFydCI6MjE3LCJlbmQiOjI1OSwidHlwZSI6IlRlbXBsYXRlTGl0ZXJhbCIsImV4cHJlc3Npb25zIjpbeyJpZCI6NjEsImFzdF90cmFpbiI6WzU5LDYwXSwic3RhcnQiOjIzNiwiZW5kIjoyNTUsInR5cGUiOiJCaW5hcnlFeHByZXNzaW9uIiwiYmluYXJ5T3BlcmF0b3IiOiItIiwibGVmdCI6eyJpZCI6NTksImFzdF90cmFpbiI6W10sInN0YXJ0IjoyMzYsImVuZCI6MjQzLCJ0eXBlIjoiSWRlbnRpZmllciIsIm5hbWUiOiJlbmRUaW1lIn0sInJpZ2h0Ijp7ImlkIjo2MCwiYXN0X3RyYWluIjpbXSwic3RhcnQiOjI0NiwiZW5kIjoyNTUsInR5cGUiOiJJZGVudGlmaWVyIiwibmFtZSI6InN0YXJ0VGltZSJ9fV0sInF1YXNpcyI6W3siaWQiOjU4LCJhc3RfdHJhaW4iOm51bGwsInN0YXJ0IjoyMTgsImVuZCI6MjM0LCJ0eXBlIjoiVGVtcGxhdGVFbGVtZW50IiwidmFsdWUiOnsiUmF3IjoiRXhlY3V0aW9uIFRpbWU6ICIsIkNvb2tlZCI6IkV4ZWN1dGlvbiBUaW1lOiAifX0seyJpZCI6NjIsImFzdF90cmFpbiI6bnVsbCwic3RhcnQiOjI1NiwiZW5kIjoyNTgsInR5cGUiOiJUZW1wbGF0ZUVsZW1lbnQiLCJ0YWlsIjp0cnVlLCJ2YWx1ZSI6eyJSYXciOiJtcyIsIkNvb2tlZCI6Im1zIn19XX1dfX1dfX0=`)),
                        };
                    
                    pageState = "done";
                    return;
                }

                /*
                if (json.job_status === "Failed") {
                    jobId = null;
                    interpretResult = json;
                    pageState = "done";
                    return;
                }
                    */
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

    $effect(() => {
        view.dispatch({
            effects: addHighlight.of(highlight)
        });
    })

    async function submitCode() {
        const src = view.state.doc.toString();
        pageState = "submitting";

        jobId = "123"

        /*
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
            */
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
        class="w-1/2 h-full bg-white border border-slate-200 rounded-xl shadow-sm overflow-scroll flex flex-col"
    >
        <div
            class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600"
        >
            Output
        </div>

        {#if showResults}
            <div class="flex flex-col w-full">
                <div class="p-6 text-slate-500 whitespace-pre-wrap">
                    {interpretResult!.result?.output}
                </div>
                <div class="flex flex-row w-full p-2 gap-2">
                    <div class="p-2 w-1/2 flex flex-col gap-2">
                        <div
                            class="bg-slate-100 px-4 py-2 border-slate-200 rounded-md text-sm font-semibold text-slate-600"
                        >
                            ECMATree
                        </div>
                        <AstTree node={interpretResult!.result!.ast} highlight={highlight} setHighlight={setHighlight}/>
                    </div>
                    <div class="p-2 w-1/2 flex flex-col gap-2">
                        <div
                            class="bg-slate-100 px-4 py-2 border-slate-200 rounded-md text-sm font-semibold text-slate-600"
                        >
                            Byte Code
                        </div>
                        <ByteCode code={interpretResult!.result!.code} highlight={highlight} setHighlight={setHighlight}/>
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
