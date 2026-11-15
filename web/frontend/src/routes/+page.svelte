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
    import { fade, fly, slide } from "svelte/transition";
    import { defaultExample, examples, type CodeExample } from "$lib/examples";
    import type {
        AstNode,
        DisassembledChunk,
        HighlightOptions,
        HighlightStatus,
        PageStatus,
        InterpretDetails,
        InterpretResult,
        JobStatus,
        ScriptJob,
        SetHighlightStatusParams,
    } from "../types";
    import AstTree from "./AstTree.svelte";
    import { generateLookUp } from "$lib/util";
    import Code from "./Code.svelte";
    import ExecutionAnimation from "./ExecutionAnimation.svelte";
    import { createSessionBootstrap, resolveUrl } from "$lib/agentSession";
    import { fetchRandomScript, fetchSessionScripts } from "$lib/scriptsApi";
    import {
        fetchJobResult,
        fetchJobStatus,
        JobApiError,
        JobPendingError,
        submitJob,
        type JobStatusResponse,
    } from "$lib/jobsApi";
    import {
        connectAgentSocket,
        type AgentSocket,
        type ConnectionStatus,
        type LastJobInfo,
        type MCPStatus,
        type ServerMessage,
    } from "$lib/agentSocket";

    const readOnlyCompartment = new Compartment();
    const astElements = $state(new Map<number, HTMLElement>());
    const lookUp = $state(new Map<number, AstNode>());

    const CARD_CLASS =
        "min-h-0 bg-white border border-slate-200 rounded-xl shadow-sm flex flex-col overflow-hidden";
    const CARD_HEADER_CLASS =
        "h-9 bg-slate-100 px-4 border-b border-slate-200 text-sm font-semibold text-slate-600 shrink-0 flex items-center justify-between gap-2";
    const ACTIVE_JOB_KEY = "go-js-active-job-v1";

    type StoredActiveJob = {
        client_request_id: string;
        request_id?: string;
        job_id?: string;
    };

    type ActiveJob = {
        jobId: string;
        requestId: string;
        clientRequestId: string;
        version: number;
        status: JobStatus;
    };

    type EditorTab = "editor" | "examples";
    type ResultView = "result" | "animation";

    let editorContainer: HTMLElement;
    let view: EditorView;
    let editorTab = $state<EditorTab>("editor");
    let activeJob = $state<ActiveJob | null>(null);
    let pendingClientRequestId = $state<string | null>(null);
    let pageState = $state<PageStatus>("input");
    let interpretResult = $state<InterpretResult | null>(null);
    let debugChunks = $state<DisassembledChunk[]>([]);
    let runError = $state<string | null>(null);
    let serverStatus = $state<ConnectionStatus>("connecting");
    let mcpStatus = $state<MCPStatus>("waiting");
    let mcpLastSeen = $state<string | null>(null);
    let mcpUrl = $state("");
    let copyState = $state<"idle" | "copied" | "failed">("idle");
    let socket: AgentSocket | null = null;
    let runTimeout: ReturnType<typeof setTimeout> | null = null;
    let jobPollTimer: ReturnType<typeof setTimeout> | null = null;
    let jobPolling = false;
    let resultAbort: AbortController | null = null;
    let sessionId = $state<string | null>(null);
    let scripts = $state<ScriptJob[]>([]);
    let scriptsLoading = $state(false);
    let scriptsError = $state<string | null>(null);
    let selectedScriptId = $state<string | null>(null);
    let randomLoading = $state(false);
    let scriptsRequest = 0;
    let runDots = $state(0);
    let outputExpanded = $state(true);
    let resultView = $state<ResultView>("result");
    let isBusy = $derived(
        pageState === "submitting" ||
            pageState === "running" ||
            pageState === "loading",
    );
    let showResults = $derived(
        pageState === "done" &&
            interpretResult !== null &&
            interpretResult.jobStatus === "Success" &&
            interpretResult.result !== null,
    );
    let animationFrameCount = $derived(
        interpretResult?.result?.execution?.frames?.length ?? 0,
    );
    let statusLabel = $derived(
        pageState === "submitting"
            ? `Submitting${".".repeat(runDots + 1)}`
            : pageState === "running"
              ? `Running${".".repeat(runDots + 1)}`
              : pageState === "loading"
                ? `Loading result${".".repeat(runDots + 1)}`
                : runError !== null
                  ? "Run failed"
                  : "",
    );
    const reveal = new Tween(0, { duration: 350, easing: cubicOut });

    $effect(() => {
        if (!isBusy) {
            runDots = 0;
            return;
        }

        const interval = setInterval(() => {
            runDots = (runDots + 1) % 3;
        }, 350);

        return () => clearInterval(interval);
    });

    $effect(() => {
        reveal.target = showResults ? 1 : 0;
    });

    let highlight = $state.raw<HighlightStatus | null>(null);
    let highlightAutoScroll = $state(true);
    let highlightScrollRequest = $state(0);
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
            highlight.chunkId === opts.chunkId &&
            highlight.opCodePtr === opts.opCodePtr
        );
    };

    const setHighlight = (
        opts: SetHighlightStatusParams,
        options: HighlightOptions = {},
    ) => {
        const shouldScroll = options.scroll ?? true;
        const shouldToggle = options.toggle ?? true;
        highlightAutoScroll = shouldScroll;

        if (opts === null || (shouldToggle && isSameHighlight(opts))) {
            highlight = null;
            return;
        }

        if (isSameHighlight(opts)) {
            if (shouldScroll) {
                highlightScrollRequest += 1;
            }
            return;
        }

        if (opts.source === "asm") {
            highlight = {
                source: "asm",
                astId: opts.astId,
                astIds: [opts.astId],
                opCodePtr: opts.opCodePtr,
                chunkId: opts.chunkId,
            };
            return;
        }

        highlight = {
            source: opts.source,
            astId: opts.astId,
            astIds: [opts.astId, ...(lookUp?.get(opts.astId)?.ast_train ?? [])],
            chunkId: opts.source === "op_code" ? opts.chunkId : "",
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
            doc: defaultExample,
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
        let cancelled = false;

        void (async () => {
            try {
                const bootstrap = await createSessionBootstrap();
                if (cancelled) {
                    return;
                }
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
                        if (status === "disconnected") {
                            maybeStartJobPolling();
                        }
                    },
                });
            } catch (error) {
                if (cancelled) {
                    return;
                }
                console.error(error);
                serverStatus = "disconnected";
                runError =
                    "Could not initialize the editor session. Reload the page to try again.";
                pageState = "error";
            }
        })();

        return () => {
            cancelled = true;
            socket?.close();
            socket = null;
            clearRunTimeout();
            stopJobPolling();
            resultAbort?.abort();
            resultAbort = null;
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
            case "loading":
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
        const autoScroll = highlightAutoScroll;
        void highlightScrollRequest;

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
                if (highlight === null || !autoScroll) {
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

            runError = "Timed out waiting for the interpreter.";
            if (activeJob !== null) {
                startJobPolling();
            }
        }, 120_000);
    }

    function readStoredJob(): StoredActiveJob | null {
        if (typeof window === "undefined") {
            return null;
        }

        try {
            const raw = sessionStorage.getItem(ACTIVE_JOB_KEY);
            if (raw === null) {
                return null;
            }
            const stored = JSON.parse(raw) as Partial<StoredActiveJob>;
            if (
                typeof stored.client_request_id !== "string" ||
                stored.client_request_id === ""
            ) {
                return null;
            }
            return {
                client_request_id: stored.client_request_id,
                request_id:
                    typeof stored.request_id === "string"
                        ? stored.request_id
                        : undefined,
                job_id:
                    typeof stored.job_id === "string"
                        ? stored.job_id
                        : undefined,
            };
        } catch {
            return null;
        }
    }

    function writeStoredJob(job: StoredActiveJob) {
        if (typeof window === "undefined") {
            return;
        }

        try {
            sessionStorage.setItem(ACTIVE_JOB_KEY, JSON.stringify(job));
        } catch {
            return;
        }
    }

    function clearStoredJob() {
        if (typeof window === "undefined") {
            return;
        }

        try {
            sessionStorage.removeItem(ACTIVE_JOB_KEY);
        } catch {
            return;
        }
    }

    function storedJobMatches(
        jobId: string,
        clientRequestId: string | undefined = undefined,
    ): boolean {
        const stored = readStoredJob();
        if (stored === null) {
            return false;
        }
        if (stored.job_id !== undefined && stored.job_id === jobId) {
            return true;
        }
        return (
            clientRequestId !== undefined &&
            stored.client_request_id === clientRequestId
        );
    }

    function stopJobPolling() {
        jobPolling = false;
        if (jobPollTimer !== null) {
            clearTimeout(jobPollTimer);
            jobPollTimer = null;
        }
    }

    function currentActiveJob(): ActiveJob | null {
        return activeJob;
    }

    function currentPageState(): PageStatus {
        return pageState;
    }

    function startJobPolling() {
        const id = sessionId;
        if (id === null || jobPolling) {
            return;
        }

        jobPolling = true;
        const tick = async () => {
            jobPollTimer = null;
            const job = activeJob;
            if (job === null) {
                jobPolling = false;
                return;
            }
            if (job.status !== "Pending" && job.status !== "Processing") {
                jobPolling = false;
                return;
            }

            try {
                const response = await fetchJobStatus(id, job.jobId);
                applyJobStatus(response);
            } catch (error) {
                console.error(error);
            }

            const next = activeJob;
            if (
                jobPolling &&
                next !== null &&
                (next.status === "Pending" || next.status === "Processing")
            ) {
                jobPollTimer = setTimeout(() => {
                    void tick();
                }, 1_000);
            } else {
                jobPolling = false;
            }
        };

        jobPollTimer = setTimeout(() => {
            void tick();
        }, 1_000);
    }

    function maybeStartJobPolling() {
        if (serverStatus === "connected") {
            stopJobPolling();
            return;
        }
        startJobPolling();
    }

    async function loadJobResult(
        jobIdToLoad: string,
        resultUrl: string | null | undefined,
    ) {
        resultAbort?.abort();
        const controller = new AbortController();
        resultAbort = controller;

        const id = sessionId;
        const url =
            typeof resultUrl === "string" && resultUrl !== ""
                ? resultUrl
                : id !== null
                  ? `/session/${encodeURIComponent(id)}/jobs/${encodeURIComponent(jobIdToLoad)}/result`
                  : null;
        if (url === null) {
            runError = "Could not load the execution result.";
            pageState = "error";
            return;
        }

        pageState = "loading";

        for (let attempt = 0; attempt < 6; attempt += 1) {
            if (controller.signal.aborted) {
                return;
            }

            try {
                const details = (await fetchJobResult(url)) as InterpretDetails;
                if (controller.signal.aborted) {
                    return;
                }

                clearRunTimeout();
                runError = null;
                debugChunks = details.debug_chunks ?? [];
                interpretResult = {
                    jobStatus: "Success",
                    result: details,
                };
                generateLookUp({
                    node: details.ast,
                    target: lookUp,
                });
                outputExpanded = true;
                resultView = "result";
                pageState = "done";
                pendingClientRequestId = null;
                activeJob = null;
                stopJobPolling();
                resultAbort = null;
                return;
            } catch (error) {
                if (controller.signal.aborted) {
                    return;
                }
                if (error instanceof JobPendingError) {
                    await new Promise((resolve) =>
                        setTimeout(resolve, error.retryAfterMs),
                    );
                    continue;
                }

                console.error(error);
                clearRunTimeout();
                activeJob = null;
                pendingClientRequestId = null;
                resultAbort = null;
                stopJobPolling();
                debugChunks = [];
                interpretResult = {
                    jobStatus: "Failed",
                    result: null,
                };
                if (
                    error instanceof JobApiError &&
                    error.code === "execution_failed"
                ) {
                    runError =
                        error.message ||
                        "The interpreter failed to run your code.";
                    pageState = "done";
                    return;
                }

                runError = "Could not load the execution result.";
                pageState = "error";
                return;
            }
        }

        clearRunTimeout();
        activeJob = null;
        pendingClientRequestId = null;
        resultAbort = null;
        stopJobPolling();
        debugChunks = [];
        interpretResult = {
            jobStatus: "Failed",
            result: null,
        };
        runError = "The execution result is taking too long to load.";
        pageState = "error";
    }

    function applyJobStatus(
        response: JobStatusResponse,
        options: { submittedByThisTab?: boolean } = {},
    ) {
        const incomingClientID = response.client_request_id ?? "";
        const matchesPending =
            pendingClientRequestId !== null &&
            incomingClientID !== "" &&
            incomingClientID === pendingClientRequestId;
        const matchesActive =
            activeJob !== null && activeJob.jobId === response.job_id;

        if (
            !options.submittedByThisTab &&
            !matchesPending &&
            !matchesActive
        ) {
            return;
        }

        let created = false;
        if (activeJob === null) {
            if (!options.submittedByThisTab && !matchesPending) {
                return;
            }
            activeJob = {
                jobId: response.job_id,
                requestId: response.request_id ?? "",
                clientRequestId:
                    incomingClientID || pendingClientRequestId || "",
                version: response.version,
                status: response.job_status,
            };
            created = true;
        } else if (activeJob.jobId !== response.job_id) {
            return;
        }

        if (!created && response.version < activeJob.version) {
            return;
        }
        if (
            !created &&
            response.version === activeJob.version &&
            response.job_status === activeJob.status
        ) {
            if (matchesPending) {
                pendingClientRequestId = null;
            }
            return;
        }

        activeJob = {
            ...activeJob,
            requestId: response.request_id ?? activeJob.requestId,
            clientRequestId:
                incomingClientID || activeJob.clientRequestId,
            version: response.version,
            status: response.job_status,
        };
        if (matchesPending) {
            pendingClientRequestId = null;
        }
        selectedScriptId = activeJob.jobId;
        writeStoredJob({
            client_request_id: activeJob.clientRequestId,
            request_id: activeJob.requestId || undefined,
            job_id: activeJob.jobId,
        });

        switch (response.job_status) {
            case "Pending":
                pageState = "running";
                maybeStartJobPolling();
                void loadSessionScripts();
                break;
            case "Processing":
                pageState = "running";
                maybeStartJobPolling();
                break;
            case "Success":
                clearRunTimeout();
                pageState = "loading";
                pendingClientRequestId = null;
                stopJobPolling();
                void loadSessionScripts();
                void loadJobResult(activeJob.jobId, response.result_url);
                break;
            case "Failed":
                clearRunTimeout();
                runError =
                    response.error?.message ||
                    "The interpreter failed to run your code.";
                debugChunks = [];
                interpretResult = {
                    jobStatus: "Failed",
                    result: null,
                };
                activeJob = null;
                pendingClientRequestId = null;
                stopJobPolling();
                pageState = "done";
                void loadSessionScripts();
                break;
        }
    }

    function handleRunStatus(
        message: Extract<ServerMessage, { type: "run.status" }>,
    ) {
        const incomingClientID = message.client_request_id ?? "";
        const matchesPending =
            pendingClientRequestId !== null &&
            incomingClientID !== "" &&
            incomingClientID === pendingClientRequestId;
        const matchesActive =
            activeJob !== null && activeJob.jobId === message.job_id;
        if (!matchesPending && !matchesActive) {
            return;
        }

        applyJobStatus({
            job_id: message.job_id,
            request_id: message.request_id,
            client_request_id: message.client_request_id,
            job_status: message.job_status,
            version: message.version,
            result_url: message.result_url,
            error:
                message.error !== undefined
                    ? {
                          code: "execution_failed",
                          message: message.error,
                      }
                    : undefined,
        });
    }

    function handleLastJob(last: LastJobInfo) {
        const matchesStored = storedJobMatches(
            last.job_id,
            last.client_request_id,
        );
        const matchesPending =
            pendingClientRequestId !== null &&
            last.client_request_id === pendingClientRequestId;
        if (!matchesStored && !matchesPending) {
            return;
        }

        if (last.job_status === "Success") {
            activeJob = {
                jobId: last.job_id,
                requestId: "",
                clientRequestId: last.client_request_id ?? "",
                version: last.version,
                status: "Success",
            };
            pendingClientRequestId = null;
            selectedScriptId = last.job_id;
            pageState = "loading";
            clearRunTimeout();
            writeStoredJob({
                client_request_id: activeJob.clientRequestId,
                job_id: activeJob.jobId,
            });
            void loadJobResult(last.job_id, last.result_url);
            void loadSessionScripts();
            return;
        }

        if (last.job_status === "Failed") {
            activeJob = null;
            pendingClientRequestId = null;
            selectedScriptId = last.job_id;
            clearRunTimeout();
            runError =
                last.error?.trim() ||
                "The interpreter failed to run your code.";
            debugChunks = [];
            interpretResult = {
                jobStatus: "Failed",
                result: null,
            };
            pageState = "done";
            void loadSessionScripts();
            return;
        }

        activeJob = {
            jobId: last.job_id,
            requestId: "",
            clientRequestId: last.client_request_id ?? "",
            version: last.version,
            status: last.job_status,
        };
        pendingClientRequestId = null;
        selectedScriptId = last.job_id;
        pageState = "running";
        armRunTimeout();
        writeStoredJob({
            client_request_id: activeJob.clientRequestId,
            job_id: activeJob.jobId,
        });
        startJobPolling();
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

    function replaceEditorSource(src: string): boolean {
        if (view === undefined || isBusy) {
            return false;
        }

        setEditorTab("editor");
        resetPage();
        view.dispatch({
            changes: {
                from: 0,
                to: view.state.doc.length,
                insert: src,
            },
        });
        view.focus();
        return true;
    }

    function applyScript(script: ScriptJob) {
        if (!replaceEditorSource(script.src)) {
            return false;
        }

        selectedScriptId = script.job_id;
        return true;
    }

    function loadExample(example: CodeExample) {
        if (!replaceEditorSource(example.source)) {
            return;
        }

        selectedScriptId = null;
    }

    function setEditorTab(tab: EditorTab) {
        if (editorTab === tab) {
            return;
        }

        editorTab = tab;
        if (tab === "editor") {
            void tick().then(() => view?.requestMeasure());
        }
    }

    function editorTabClass(tab: EditorTab): string {
        return editorTab === tab
            ? "cursor-pointer rounded-md bg-white px-3 py-1 text-xs font-semibold text-slate-700 shadow-sm"
            : "cursor-pointer rounded-md px-3 py-1 text-xs font-medium text-slate-500 hover:text-slate-700";
    }

    function resultTabClass(tab: ResultView): string {
        return resultView === tab
            ? "cursor-pointer rounded-md bg-white px-3 py-1 text-xs font-semibold text-slate-700 shadow-sm"
            : "cursor-pointer rounded-md px-3 py-1 text-xs font-medium text-slate-500 hover:text-slate-700";
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

    function rejectExecute(requestID: string | undefined, message: string) {
        if (requestID === undefined) {
            return;
        }
        socket?.send({
            type: "execute.rejected",
            request_id: requestID,
            error: message,
        });
    }

    async function handleExecuteRequest(message: {
        request_id: string;
        code: string;
    }) {
        if (view === undefined) {
            rejectExecute(
                message.request_id,
                "The editor is not ready yet.",
            );
            return;
        }

        if (
            activeJob !== null ||
            pageState === "submitting" ||
            pageState === "running" ||
            pageState === "loading"
        ) {
            rejectExecute(
                message.request_id,
                "The editor is busy running another request.",
            );
            return;
        }

        resetPage();
        setEditorTab("editor");
        view.dispatch({
            changes: {
                from: 0,
                to: view.state.doc.length,
                insert: message.code,
            },
        });
        await tick();

        await sendRun(message.code, message.request_id);
    }

    function handleServerMessage(message: ServerMessage) {
        switch (message.type) {
            case "session.ready": {
                mcpStatus = message.mcp_status;
                mcpLastSeen = message.mcp_last_seen ?? null;

                const active = message.active_run;
                if (active?.job_id !== undefined) {
                    const matchesPending =
                        pendingClientRequestId !== null &&
                        active.client_request_id === pendingClientRequestId;
                    const matchesStored = storedJobMatches(
                        active.job_id,
                        active.client_request_id,
                    );
                    if (matchesPending || matchesStored) {
                        activeJob = {
                            jobId: active.job_id,
                            requestId: active.request_id,
                            clientRequestId:
                                active.client_request_id ?? "",
                            version: active.version ?? 1,
                            status: active.job_status ?? "Processing",
                        };
                        pendingClientRequestId = null;
                        selectedScriptId = active.job_id;
                        pageState = "running";
                        armRunTimeout();
                        stopJobPolling();
                        writeStoredJob({
                            client_request_id:
                                activeJob.clientRequestId,
                            request_id: activeJob.requestId || undefined,
                            job_id: activeJob.jobId,
                        });
                        void loadSessionScripts();
                    }
                    return;
                }

                if (message.last_job !== undefined) {
                    handleLastJob(message.last_job);
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
            case "run.status":
                handleRunStatus(message);
                return;
        }
    }

    function submitCode(): void {
        void sendRun(view.state.doc.toString());
    }

    async function sendRun(
        code: string,
        executeRequestId: string | undefined = undefined,
    ): Promise<boolean> {
        if (
            activeJob !== null ||
            pageState === "submitting" ||
            pageState === "running" ||
            pageState === "loading"
        ) {
            rejectExecute(
                executeRequestId,
                "The editor is busy running another request.",
            );
            return false;
        }
        const id = sessionId;
        if (id === null) {
            runError =
                "The editor session is not ready. Reload the page and try again.";
            pageState = "error";
            rejectExecute(
                executeRequestId,
                "The editor session is not ready.",
            );
            return false;
        }

        const clientRequestId = executeRequestId ?? crypto.randomUUID();
        runError = null;
        interpretResult = null;
        debugChunks = [];
        activeJob = null;
        resultView = "result";
        highlight = null;
        astElements.clear();
        lookUp.clear();
        pendingClientRequestId = clientRequestId;
        pageState = "submitting";
        armRunTimeout();
        writeStoredJob({
            client_request_id: clientRequestId,
            request_id: executeRequestId,
        });

        try {
            const response = await submitJob(id, {
                code,
                client_request_id: clientRequestId,
                ...(executeRequestId !== undefined
                    ? { execute_request_id: executeRequestId }
                    : {}),
            });
            applyJobStatus(response, { submittedByThisTab: true });
            return true;
        } catch (error) {
            console.error(error);
            const adoptedJob = currentActiveJob();
            const state = currentPageState();
            if (
                (adoptedJob !== null &&
                    adoptedJob.clientRequestId === clientRequestId) ||
                state === "loading" ||
                state === "done"
            ) {
                maybeStartJobPolling();
                return true;
            }

            const message =
                error instanceof JobApiError
                    ? error.message
                    : "Could not submit your code. Please try again.";
            clearRunTimeout();
            pendingClientRequestId = null;
            clearStoredJob();
            stopJobPolling();
            runError = message;
            interpretResult = null;
            pageState = "error";
            rejectExecute(executeRequestId, message);
            return false;
        }
    }

    function resetPage() {
        clearRunTimeout();
        stopJobPolling();
        resultAbort?.abort();
        resultAbort = null;
        pageState = "input";
        activeJob = null;
        pendingClientRequestId = null;
        clearStoredJob();
        interpretResult = null;
        debugChunks = [];
        runError = null;
        highlight = null;
        resultView = "result";
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
            class="w-64 shrink-0 {CARD_CLASS}"
            aria-label="Latest runs"
        >
            <div class={CARD_HEADER_CLASS}>
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
                                    onclick={() => applyScript(script)}
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
            class="basis-0 min-w-0 {CARD_CLASS}"
            style:flex-grow={2 - reveal.current}
        >
            <div
                class="{CARD_HEADER_CLASS} flex-row"
            >
                <div
                    class="flex items-center gap-1 rounded-lg bg-slate-200/70 p-1"
                    role="tablist"
                    aria-label="Editor panels"
                >
                    <button
                        type="button"
                        role="tab"
                        aria-selected={editorTab === "editor"}
                        class={editorTabClass("editor")}
                        onclick={() => setEditorTab("editor")}
                    >
                        Editor
                    </button>
                    <button
                        type="button"
                        role="tab"
                        aria-selected={editorTab === "examples"}
                        class={editorTabClass("examples")}
                        onclick={() => setEditorTab("examples")}
                    >
                        Examples
                    </button>
                </div>
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

            <div
                class="flex-1 min-h-0 overflow-y-auto relative"
                class:hidden={editorTab === "examples"}
            >
                <div
                    bind:this={editorContainer}
                    class="absolute inset-0 p-2"
                ></div>
            </div>

            {#if editorTab === "examples"}
                <div class="flex-1 min-h-0 overflow-y-auto p-4">
                    <div class="grid grid-cols-1 gap-3 xl:grid-cols-2">
                        {#each examples as example (example.id)}
                            <button
                                type="button"
                                onclick={() => loadExample(example)}
                                disabled={isBusy}
                                class="group flex cursor-pointer flex-col gap-2 rounded-xl border border-slate-200 bg-slate-50 p-4 text-left transition-colors hover:border-[#00ADD8] hover:bg-cyan-50 disabled:cursor-not-allowed disabled:opacity-50"
                            >
                                <span
                                    class="flex w-full items-start justify-between gap-3"
                                >
                                    <span
                                        class="text-sm font-semibold text-slate-700 group-hover:text-[#00758D]"
                                    >
                                        {example.title}
                                    </span>
                                    <span
                                        class="shrink-0 rounded-full border border-slate-200 bg-white px-2 py-0.5 text-[10px] font-medium text-slate-500"
                                    >
                                        {example.source.split("\n").length}
                                        lines
                                    </span>
                                </span>
                                <span
                                    class="text-xs leading-relaxed text-slate-500"
                                >
                                    {example.description}
                                </span>
                                <span
                                    class="mt-auto font-mono text-[11px] text-slate-400"
                                >
                                    Click to load into the editor
                                </span>
                            </button>
                        {/each}
                    </div>
                </div>
            {/if}

            <div
                class="p-4 border-t border-slate-100 flex flex-row flex-wrap gap-2 justify-between shrink-0"
            >
                <button
                    onclick={() => submitCode()}
                    disabled={pageState !== "input" || sessionId === null}
                    class="inline-flex min-w-28 items-center justify-center gap-2 bg-[#00ADD8] hover:bg-[#00758D] text-white px-4 py-2 rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                    {#if isBusy}
                        <span
                            class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white"
                            aria-hidden="true"
                        ></span>
                        {pageState === "submitting"
                            ? "Submitting"
                            : pageState === "loading"
                              ? "Loading"
                              : "Running"}
                    {:else}
                        Submit
                    {/if}
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
            class="basis-0 min-w-0 min-h-0 flex flex-col {showResults
                ? ''
                : 'pointer-events-none'}"
            style:flex-grow={2 * reveal.current}
            style:margin-left={`${reveal.current}rem`}
            style:opacity={reveal.current}
            aria-hidden={!showResults}
        >
            {#if showResults}
                <div
                    class="flex w-full flex-1 min-h-0 flex-col gap-3"
                    in:fly={{ x: 32, duration: 300, easing: cubicOut }}
                    out:fade={{ duration: 150 }}
                >
                    <section class="shrink-0 {CARD_CLASS}">
                        <div class="{CARD_HEADER_CLASS}">
                            {#if resultView === "result"}
                                <button
                                    type="button"
                                    class="flex min-w-0 cursor-pointer items-center gap-1 text-left hover:text-slate-800"
                                    aria-expanded={outputExpanded}
                                    aria-controls="execution-result-panel"
                                    onclick={() =>
                                        (outputExpanded = !outputExpanded)}
                                >
                                    <span>Execution result</span>
                                    <span
                                        class="inline-block transition-transform {outputExpanded
                                            ? 'rotate-90'
                                            : ''}"
                                        aria-hidden="true"
                                    >
                                        ▸
                                    </span>
                                </button>
                            {:else}
                                <span
                                    class="flex min-w-0 items-center gap-2"
                                >
                                    <span>Execution animation</span>
                                    {#if animationFrameCount > 0}
                                        <span
                                            class="rounded-full bg-cyan-50 px-1.5 py-0.5 text-[9px] font-semibold text-cyan-600"
                                        >
                                            {animationFrameCount} frames
                                        </span>
                                    {/if}
                                </span>
                            {/if}

                            <div
                                class="flex items-center gap-1 rounded-lg bg-slate-200/70 p-1"
                                role="tablist"
                                aria-label="Execution view"
                            >
                                <button
                                    type="button"
                                    role="tab"
                                    aria-selected={resultView === "result"}
                                    class={resultTabClass("result")}
                                    onclick={() => (resultView = "result")}
                                >
                                    Result
                                </button>
                                <button
                                    type="button"
                                    role="tab"
                                    aria-selected={resultView === "animation"}
                                    class="{resultTabClass(
                                        'animation',
                                    )} inline-flex items-center"
                                    onclick={() => (resultView = "animation")}
                                >
                                    Animation
                                    {#if animationFrameCount > 0}
                                        <span
                                            class="ml-1 h-1.5 w-1.5 rounded-full bg-cyan-500"
                                            aria-hidden="true"
                                        ></span>
                                    {/if}
                                </button>
                            </div>
                        </div>

                        {#if resultView === "result" && outputExpanded}
                            <div
                                id="execution-result-panel"
                                class="max-h-[35vh] overflow-y-auto p-3"
                                transition:slide={{ duration: 200 }}
                            >
                                <pre
                                    class="whitespace-pre-wrap break-words font-mono text-sm text-slate-600">{interpretResult!
                                        .result?.output || "(no output)"}</pre>
                            </div>
                        {/if}
                    </section>

                    {#if resultView === "result"}
                        <div class="flex min-h-0 flex-1 flex-row gap-3">
                            <section
                                class="flex min-w-0 flex-1 flex-col {CARD_CLASS}"
                            >
                                <div class={CARD_HEADER_CLASS}>ECMATree</div>
                                <div class="flex-1 min-h-0 overflow-y-auto p-2">
                                    <AstTree
                                        node={interpretResult!.result!.ast}
                                        {highlight}
                                        {setHighlight}
                                        {registerThySelf}
                                    />
                                </div>
                            </section>

                            <section
                                class="flex min-w-0 flex-1 flex-col {CARD_CLASS}"
                            >
                                <div class={CARD_HEADER_CLASS}>Byte Code</div>
                                <div
                                    class="flex flex-1 min-h-0 flex-col p-2"
                                >
                                    <Code
                                        chunks={debugChunks}
                                        {highlight}
                                        {setHighlight}
                                    />
                                </div>
                            </section>
                        </div>
                    {:else}
                        <section
                            class="flex min-h-0 flex-1 flex-col {CARD_CLASS}"
                        >
                            <div class="min-h-0 flex-1">
                                <ExecutionAnimation
                                    execution={interpretResult!.result
                                        ?.execution ?? null}
                                    output={interpretResult!.result?.output ??
                                        ""}
                                    chunks={debugChunks}
                                    {highlight}
                                    {setHighlight}
                                />
                            </div>
                        </section>
                    {/if}
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
