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
    import { onMount, tick } from "svelte";
    import { Tween } from "svelte/motion";
    import { cubicOut } from "svelte/easing";
    import { fade, fly } from "svelte/transition";
    import fibo from "$lib/examples/fibonacci?raw";
    import type {
        AstNode,
        HighlightStatus,
        PageStatus,
        InterpretResult,
        JobStatus,
        ScriptJob,
        SetHighlightStatusParams,
    } from "../types";
    import AstTree from "./AstTree.svelte";
    import { generateLookUp } from "$lib/util";
    import Code from "./Code.svelte";
    import { createSessionBootstrap, resolveUrl } from "$lib/agentSession";
    import { fetchRandomScript, fetchSessionScripts } from "$lib/scriptsApi";
    import {
        connectAgentSocket,
        type AgentSocket,
        type ConnectionStatus,
        type MCPStatus,
        type ServerMessage,
    } from "$lib/agentSocket";

    const readOnlyCompartment = new Compartment();
    const astElements = $state(new Map<number, HTMLElement>());
    const lookUp = $state(new Map<number, AstNode>());

    let editorContainer: HTMLElement;
    let view: EditorView;
    let jobId = $state<null | string>(null);
    let activeRequestId = $state<null | string>(null);
    let pageState = $state<PageStatus>("input");
    let interpretResult = $state<InterpretResult | null>(null);
    let runError = $state<string | null>(null);
    let serverStatus = $state<ConnectionStatus>("connecting");
    let mcpStatus = $state<MCPStatus>("waiting");
    let mcpLastSeen = $state<string | null>(null);
    let mcpUrl = $state("");
    let copyState = $state<"idle" | "copied" | "failed">("idle");
    let socket: AgentSocket | null = null;
    let runTimeout: ReturnType<typeof setTimeout> | null = null;
    let sessionId = $state<string | null>(null);
    let scripts = $state<ScriptJob[]>([]);
    let scriptsLoading = $state(false);
    let scriptsError = $state<string | null>(null);
    let selectedScriptId = $state<string | null>(null);
    let randomLoading = $state(false);
    let scriptsRequest = 0;
    let isBusy = $derived(
        pageState === "submitting" || pageState === "running",
    );
    let showResults = $derived(
        pageState === "done" &&
            interpretResult !== null &&
            interpretResult.jobStatus === "Success" &&
            interpretResult.result !== null,
    );
    let statusLabel = $derived(
        pageState === "submitting"
            ? "Submitting…"
            : pageState === "running"
              ? "Running…"
              : runError !== null
                ? "Run failed"
                : "",
    );
    const reveal = new Tween(0, { duration: 350, easing: cubicOut });

    $effect(() => {
        reveal.target = showResults ? 1 : 0;
    });

    let highlight = $state.raw<HighlightStatus | null>(null);
    let timeout = $state.raw<number | null>(null);

    const isSameHighlight = (opts: SetHighlightStatusParams) => {
        if (highlight === null || opts === null) {
            return false;
        }

        if (
            highlight.source !== opts.source ||
            highlight.astId !== opts.astId
        ) {
            return false;
        }

        if (opts.source === "ast") {
            return true;
        }

        return (
            highlight.fn === opts.fn && highlight.opCodePtr === opts.opCodePtr
        );
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
                EditorView.lineWrapping,
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

    onMount(() => {
        const bootstrap = createSessionBootstrap();
        if (bootstrap === null) {
            serverStatus = "disconnected";
            runError =
                "Could not initialize the editor session. Reload the page to try again.";
            pageState = "error";
            return;
        }

        mcpUrl = resolveUrl(bootstrap.mcp_path);
        sessionId = bootstrap.session_id;
        void loadSessionScripts();
        socket = connectAgentSocket({
            websocketPath: bootstrap.ws_path,
            onMessage: handleServerMessage,
            onStatus: (status) => {
                serverStatus = status;
            },
        });

        return () => {
            socket?.close();
            socket = null;
            clearRunTimeout();
        };
    });

    $effect(() => {
        if (!showResults) {
            return;
        }

        const timeout = setTimeout(() => {
            view?.requestMeasure();
        }, 400);

        return () => clearTimeout(timeout);
    });

    $effect(() => {
        switch (pageState) {
            case "done":
            case "running":
            case "submitting": {
                view.dispatch({
                    effects: readOnlyCompartment.reconfigure([
                        EditorState.readOnly.of(true),
                        EditorView.editable.of(false),
                    ]),
                });
                break;
            }
            case "input":
            case "error": {
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

    function clearRunTimeout() {
        if (runTimeout !== null) {
            clearTimeout(runTimeout);
            runTimeout = null;
        }
    }

    function armRunTimeout() {
        clearRunTimeout();
        runTimeout = setTimeout(() => {
            runTimeout = null;
            if (pageState !== "submitting" && pageState !== "running") {
                return;
            }

            activeRequestId = null;
            jobId = null;
            runError = "Timed out waiting for the interpreter.";
            interpretResult = null;
            pageState = "error";
        }, 120_000);
    }

    async function loadSessionScripts() {
        const id = sessionId;
        if (id === null) {
            return;
        }

        const request = ++scriptsRequest;
        scriptsLoading = true;
        try {
            const nextScripts = await fetchSessionScripts(id);
            if (request !== scriptsRequest) {
                return;
            }
            scripts = nextScripts;
            scriptsError = null;
        } catch (error) {
            if (request !== scriptsRequest) {
                return;
            }
            console.error(error);
            scriptsError = "Could not load your latest runs.";
        } finally {
            if (request === scriptsRequest) {
                scriptsLoading = false;
            }
        }
    }

    async function loadRandomScript() {
        if (isBusy || randomLoading) {
            return;
        }

        randomLoading = true;
        try {
            const script = await fetchRandomScript();
            if (script === null) {
                scriptsError = "No scripts in the database yet.";
                return;
            }
            applyScript(script);
            scriptsError = null;
        } catch (error) {
            console.error(error);
            scriptsError = "Could not fetch a random script.";
        } finally {
            randomLoading = false;
        }
    }

    function applyScript(script: ScriptJob) {
        if (view === undefined || isBusy) {
            return false;
        }

        resetPage();
        view.dispatch({
            changes: {
                from: 0,
                to: view.state.doc.length,
                insert: script.src,
            },
        });
        selectedScriptId = script.job_id;
        view.focus();
        return true;
    }

    function runScript(script: ScriptJob) {
        if (!applyScript(script)) {
            return;
        }
        submitCode();
    }

    function scriptPreview(src: string): string {
        const line = src.split("\n").find((value) => value.trim() !== "");
        return (line ?? "Empty script").trim().slice(0, 90);
    }

    function formatScriptTime(value: string): string {
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) {
            return "";
        }

        return date.toLocaleString(undefined, {
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
        });
    }

    function statusDotClass(status: JobStatus): string {
        switch (status) {
            case "Success":
                return "bg-emerald-500";
            case "Failed":
                return "bg-red-500";
            case "Processing":
                return "bg-amber-400";
            default:
                return "bg-slate-400";
        }
    }

    function scriptButtonClass(jobId: string): string {
        return selectedScriptId === jobId
            ? "border-[#00ADD8] bg-cyan-50"
            : "border-slate-200 bg-white hover:bg-slate-50";
    }

    function handleRunAccepted(message: {
        request_id: string;
        job_id: string;
    }) {
        if (activeRequestId === null && pageState !== "submitting") {
            return;
        }
        if (
            activeRequestId !== null &&
            activeRequestId !== message.request_id
        ) {
            return;
        }

        activeRequestId = message.request_id;
        jobId = message.job_id;
        selectedScriptId = message.job_id;
        pageState = "running";
        void loadSessionScripts();
    }

    function handleRunResult(message: {
        request_id: string;
        job_id: string;
        job_status: string;
        result: string;
    }) {
        const isCurrentRun =
            activeRequestId !== null
                ? activeRequestId === message.request_id
                : (pageState === "submitting" || pageState === "running") &&
                  (jobId === null || jobId === message.job_id);
        if (!isCurrentRun) {
            return;
        }

        clearRunTimeout();
        activeRequestId = null;
        jobId = null;
        void loadSessionScripts();

        if (message.job_status === "Success") {
            try {
                const interpretDetails = JSON.parse(atob(message.result));
                runError = null;
                interpretResult = {
                    jobStatus: "Success",
                    result: interpretDetails,
                };
                generateLookUp({
                    node: interpretDetails.ast,
                    target: lookUp,
                });
                pageState = "done";
            } catch (error) {
                console.error(error);
                runError = "The interpreter returned an invalid result.";
                interpretResult = {
                    jobStatus: "Failed",
                    result: null,
                };
                pageState = "done";
            }
            return;
        }

        runError =
            typeof message.result === "string" && message.result.trim() !== ""
                ? message.result
                : "The interpreter failed to run your code.";
        interpretResult = {
            jobStatus: "Failed",
            result: null,
        };
        pageState = "done";
    }

    function handleRunError(message: { request_id: string; error: string }) {
        if (
            activeRequestId !== null &&
            activeRequestId !== message.request_id
        ) {
            return;
        }
        if (activeRequestId === null && pageState !== "submitting") {
            return;
        }

        clearRunTimeout();
        activeRequestId = null;
        jobId = null;
        runError = message.error || "The interpreter could not start your run.";
        interpretResult = null;
        pageState = "error";
    }

    async function handleExecuteRequest(message: {
        request_id: string;
        code: string;
    }) {
        if (view === undefined) {
            socket?.send({
                type: "execute.rejected",
                request_id: message.request_id,
                error: "The editor is not ready yet.",
            });
            return;
        }

        if (pageState === "submitting" || pageState === "running") {
            socket?.send({
                type: "execute.rejected",
                request_id: message.request_id,
                error: "The editor is busy running another request.",
            });
            return;
        }

        resetPage();
        view.dispatch({
            changes: {
                from: 0,
                to: view.state.doc.length,
                insert: message.code,
            },
        });
        await tick();

        submitAgentCode(message.request_id);
    }

    function handleServerMessage(message: ServerMessage) {
        switch (message.type) {
            case "session.ready": {
                mcpStatus = message.mcp_status;
                mcpLastSeen = message.mcp_last_seen ?? null;
                if (message.active_run !== undefined) {
                    activeRequestId = message.active_run.request_id;
                    jobId = message.active_run.job_id ?? null;
                    pageState = "running";
                    armRunTimeout();
                }
                return;
            }
            case "mcp.status":
                mcpStatus = message.mcp_status;
                mcpLastSeen = message.mcp_last_seen ?? null;
                return;
            case "execute.request":
                void handleExecuteRequest(message);
                return;
            case "run.accepted":
                handleRunAccepted(message);
                return;
            case "run.result":
                handleRunResult(message);
                return;
            case "run.error":
                handleRunError(message);
                return;
        }
    }

    function submitCode(): boolean {
        return sendRun(view.state.doc.toString());
    }

    function submitAgentCode(requestId: string): boolean {
        return sendRun(view.state.doc.toString(), requestId);
    }

    function sendRun(code: string, requestId: string | undefined = undefined): boolean {
        if (
            activeRequestId !== null ||
            pageState === "submitting" ||
            pageState === "running"
        ) {
            return false;
        }
        if (socket === null || serverStatus !== "connected") {
            runError =
                "The editor connection is not ready. Reconnecting, please try again.";
            pageState = "error";
            return false;
        }

        runError = null;
        interpretResult = null;
        jobId = null;
        activeRequestId = requestId ?? null;
        pageState = "submitting";
        armRunTimeout();

        const sent =
            requestId === undefined
                ? socket.send({
                      type: "run.submit",
                      code,
                  })
                : socket.send({
                      type: "run.submit",
                      request_id: requestId,
                      code,
                  });

        if (!sent) {
            clearRunTimeout();
            activeRequestId = null;
            runError = "Could not submit your code. Please try again.";
            pageState = "error";
            return false;
        }

        return true;
    }

    function resetPage() {
        clearRunTimeout();
        pageState = "input";
        jobId = null;
        activeRequestId = null;
        interpretResult = null;
        runError = null;
        highlight = null;
        astElements.clear();
        lookUp.clear();
    }

    async function copyMcpUrl() {
        try {
            await navigator.clipboard.writeText(mcpUrl);
            copyState = "copied";
        } catch {
            copyState = "failed";
        }

        setTimeout(() => {
            copyState = "idle";
        }, 1_500);
    }

    function registerThySelf({ id, el }: { id: number; el: HTMLElement }) {
        astElements.set(id, el);
    }
</script>

<div
    class="w-full h-screen overflow-hidden flex flex-col p-4 bg-slate-50 gap-3"
>
    <div
        class="shrink-0 bg-white border border-slate-200 rounded-xl shadow-sm px-4 py-3 flex flex-wrap items-center gap-3"
    >
        <span class="text-sm font-semibold text-slate-600">MCP endpoint</span>
        <code
            class="min-w-0 flex-1 truncate rounded-md bg-slate-100 px-3 py-1.5 text-xs text-slate-700"
            title={mcpUrl}>{mcpUrl || "Connecting…"}</code
        >
        <button
            type="button"
            onclick={copyMcpUrl}
            disabled={!mcpUrl}
            class="bg-[#00ADD8] hover:bg-[#00758D] text-white px-3 py-1.5 rounded-lg transition-colors text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
        >
            {copyState === "copied"
                ? "Copied"
                : copyState === "failed"
                  ? "Copy failed"
                  : "Copy"}
        </button>
        <span
            class="flex items-center gap-1.5 text-xs font-medium text-slate-500"
            aria-live="polite"
            title="Browser connection to the editor server"
        >
            <span
                class="h-2 w-2 rounded-full {serverStatus === 'connected'
                    ? 'bg-emerald-500'
                    : serverStatus === 'connecting'
                      ? 'bg-amber-400'
                      : 'bg-red-500'}"
            ></span>
            {serverStatus === "connected"
                ? "Server connected"
                : serverStatus === "connecting"
                  ? "Connecting…"
                  : "Disconnected"}
        </span>
        <span
            class="flex items-center gap-1.5 text-xs font-medium text-slate-500"
            aria-live="polite"
            title={mcpLastSeen ?? "No MCP activity yet"}
        >
            <span
                class="h-2 w-2 rounded-full {serverStatus !== 'connected'
                    ? 'bg-slate-300'
                    : mcpStatus === 'active'
                      ? 'bg-emerald-500'
                      : mcpStatus === 'idle'
                        ? 'bg-amber-400'
                        : 'bg-slate-400'}"
            ></span>
            {serverStatus !== "connected"
                ? "MCP status unknown"
                : mcpStatus === "active"
                  ? "Agent active"
                  : mcpStatus === "idle"
                    ? "Last active"
                    : "Waiting for agent"}
        </span>
    </div>

    <div class="flex-1 min-h-0 flex flex-row gap-3">
        <aside
            class="w-64 shrink-0 bg-white border border-slate-200 rounded-xl shadow-sm flex flex-col overflow-hidden"
            aria-label="Latest runs"
        >
            <div
                class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600 shrink-0 flex items-center justify-between gap-2"
            >
                <span>Latest runs</span>
                <button
                    type="button"
                    onclick={() => void loadSessionScripts()}
                    disabled={scriptsLoading || sessionId === null}
                    class="text-xs font-medium text-[#00758D] hover:text-[#00ADD8] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                    {scriptsLoading ? "Loading…" : "Refresh"}
                </button>
            </div>

            <div class="p-3 border-b border-slate-100 shrink-0">
                <button
                    type="button"
                    onclick={() => void loadRandomScript()}
                    disabled={isBusy || randomLoading}
                    class="w-full bg-[#00ADD8] hover:bg-[#00758D] text-white px-3 py-2 rounded-lg transition-colors text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                    {randomLoading
                        ? "Finding a script…"
                        : "🎲 Give me a random script"}
                </button>
            </div>

            {#if scriptsError}
                <div
                    class="mx-3 mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700"
                    role="status"
                >
                    {scriptsError}
                </div>
            {/if}

            <div class="flex-1 min-h-0 overflow-y-auto p-2">
                {#if scripts.length === 0}
                    <p class="px-2 py-3 text-xs text-slate-500">
                        {scriptsLoading
                            ? "Loading runs…"
                            : "No runs yet. Submit one and it will show up here."}
                    </p>
                {:else}
                    <ul class="flex flex-col gap-1">
                        {#each scripts as script (script.job_id)}
                            <li>
                                <button
                                    type="button"
                                    onclick={() => runScript(script)}
                                    disabled={isBusy}
                                    class="w-full text-left rounded-lg border px-3 py-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer {scriptButtonClass(
                                        script.job_id,
                                    )}"
                                >
                                    <span
                                        class="flex items-center gap-1.5 text-[11px] font-medium text-slate-500"
                                    >
                                        <span
                                            class="h-1.5 w-1.5 rounded-full {statusDotClass(
                                                script.job_status,
                                            )}"
                                        ></span>
                                        {script.job_status}
                                        <span class="ml-auto font-normal">
                                            {formatScriptTime(
                                                script.created_at,
                                            )}
                                        </span>
                                    </span>
                                    <span
                                        class="mt-1 block truncate font-mono text-xs text-slate-700"
                                    >
                                        {scriptPreview(script.src)}
                                    </span>
                                    <span
                                        class="mt-1 block font-mono text-[10px] text-slate-400"
                                    >
                                        {script.job_id.slice(0, 8)}
                                    </span>
                                </button>
                            </li>
                        {/each}
                    </ul>
                {/if}
            </div>
        </aside>

        <div class="flex-1 min-w-0 flex flex-row">
        <div
            class="basis-0 min-w-0"
            style:flex-grow={1 - reveal.current}
            aria-hidden="true"
        ></div>

        <div
            class="basis-0 min-w-0 bg-white border border-slate-200 rounded-xl shadow-sm flex flex-col overflow-hidden"
            style:flex-grow={2 - reveal.current}
        >
            <div
                class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600 shrink-0 flex flex-row items-center justify-between gap-2"
            >
                <span>Editor</span>
                {#if statusLabel}
                    <span
                        class="text-xs font-medium text-slate-500"
                        aria-live="polite"
                        transition:fade={{ duration: 150 }}
                    >
                        {statusLabel}
                    </span>
                {/if}
            </div>

            {#if runError}
                <div
                    class="mx-4 mt-4 max-h-40 overflow-y-auto whitespace-pre-wrap rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 shrink-0"
                    role="alert"
                    in:fly={{ y: -8, duration: 200, easing: cubicOut }}
                    out:fade={{ duration: 150 }}
                >
                    {runError}
                </div>
            {/if}

            <div class="flex-1 min-h-0 overflow-y-auto relative">
                <div
                    bind:this={editorContainer}
                    class="absolute inset-0 p-2"
                ></div>
            </div>

            <div
                class="p-4 border-t border-slate-100 flex flex-row flex-wrap gap-2 justify-between shrink-0"
            >
                <button
                    onclick={() => submitCode()}
                    disabled={pageState !== "input" ||
                        serverStatus !== "connected"}
                    class="bg-[#00ADD8] hover:bg-[#00758D] text-white px-4 py-2 rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                    Submit
                </button>
                {#if pageState !== "input"}
                    <button
                        onclick={resetPage}
                        class="bg-[#F3F4F6] hover:bg-[#E5E7EB] text-[#374151] px-4 py-2 rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                        transition:fade={{ duration: 150 }}
                    >
                        Reset
                    </button>
                {/if}
            </div>
        </div>

        <div
            class="basis-0 min-w-0 overflow-hidden bg-white border border-slate-200 rounded-xl shadow-sm flex flex-col {showResults
                ? ''
                : 'pointer-events-none'}"
            style:flex-grow={2 * reveal.current}
            style:margin-left={`${reveal.current}rem`}
            style:opacity={reveal.current}
            aria-hidden={!showResults}
        >
            {#if showResults}
                <div
                    class="flex flex-col w-full flex-1 min-h-0"
                    in:fly={{ x: 32, duration: 300, easing: cubicOut }}
                    out:fade={{ duration: 150 }}
                >
                    <div
                        class="bg-slate-100 px-4 py-2 border-b border-slate-200 text-sm font-semibold text-slate-600 shrink-0"
                    >
                        Results
                    </div>

                    <div
                        class="p-6 text-slate-500 whitespace-pre-wrap shrink-0"
                    >
                        {interpretResult!.result?.output}
                    </div>

                    <div class="flex flex-row w-full p-2 gap-2 flex-1 min-h-0">
                        <div
                            class="p-2 w-1/2 min-w-0 min-h-0 flex flex-col gap-2"
                        >
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
                        <div
                            class="p-2 w-1/2 min-w-0 min-h-0 flex flex-col gap-2"
                        >
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
            {/if}
        </div>

        <div
            class="basis-0 min-w-0"
            style:flex-grow={1 - reveal.current}
            aria-hidden="true"
        ></div>
        </div>
    </div>
</div>
