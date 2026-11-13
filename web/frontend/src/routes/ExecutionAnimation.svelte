<script lang="ts">
    import type {
        EventLoopState,
        ExecutionFrame,
        ExecutionFrameKind,
        ExecutionInfo,
        HighlightOptions,
        QueueState,
        SetHighlightStatusParams,
        VMState,
    } from "../types";
    import ExecutionEventLoop from "./ExecutionEventLoop.svelte";
    import ExecutionQueues from "./ExecutionQueues.svelte";
    import ExecutionStack from "./ExecutionStack.svelte";

    type Props = {
        execution?: ExecutionInfo | null;
        output?: string;
        setHighlight: (
            opts: SetHighlightStatusParams,
            options?: HighlightOptions,
        ) => void;
    };

    const { execution = null, output = "", setHighlight }: Props = $props();

    const emptyQueues: QueueState = { micro_tasks: [], tasks: [] };
    const emptyEventLoop: EventLoopState = {
        phase: "booting",
        work: 0,
        timers: [],
    };

    let frameIndex = $state(0);
    let playing = $state(false);
    let speed = $state(10);
    let scrollRequest = $state(0);
    let lastScrollRequest = 0;
    let previousExecution: ExecutionInfo | null | undefined = undefined;

    const frames = $derived(execution?.frames ?? []);
    const frameCount = $derived(frames.length);
    const lastIndex = $derived(Math.max(0, frameCount - 1));
    const currentFrame = $derived(frames[frameIndex] ?? null);

    const vmAtFrame = $derived.by<(VMState | null)[]>(() => {
        let lastVm: VMState | null = null;

        return frames.map((frame) => {
            if (frame.vm) {
                lastVm = frame.vm;
            }
            return lastVm;
        });
    });

    const outputAtFrame = $derived.by<string[]>(() => {
        let text = "";

        return frames.map((frame) => {
            if (frame.output?.delta) {
                text += frame.output.delta;
            }
            return text;
        });
    });

    const currentVm = $derived(vmAtFrame[frameIndex] ?? null);
    const rawOutput = $derived(outputAtFrame[frameIndex] ?? "");
    const atEnd = $derived(frameCount === 0 || frameIndex >= lastIndex);
    const firstJitIndex = $derived(
        frames.findIndex((frame) => frame.kind === "jit"),
    );
    const jitFrameCount = $derived(
        frames.reduce(
            (count, frame) => count + (frame.kind === "jit" ? 1 : 0),
            0,
        ),
    );
    const currentOutput = $derived(
        atEnd && output !== "" ? output : rawOutput,
    );
    const queues = $derived(currentFrame?.queues ?? emptyQueues);
    const eventLoop = $derived(currentFrame?.event_loop ?? emptyEventLoop);
    const narration = $derived(
        currentFrame === null ? "" : narrationFor(currentFrame),
    );
    const secondaryNote = $derived.by(() => {
        if (currentFrame === null) {
            return "";
        }

        const options = [currentFrame.note, currentFrame.event?.detail];
        return (
            options.find(
                (value): value is string =>
                    typeof value === "string" &&
                    value !== "" &&
                    value !== narration,
            ) ?? ""
        );
    });
    const unavailableMessage = $derived.by(() => {
        if (execution === null) {
            return "This result does not include an execution trace.";
        }

        switch (execution.status) {
            case "not_requested":
                return "Execution tracing was not requested for this run.";
            case "error":
                return "The runtime could not complete the execution trace.";
            default:
                return "The execution trace contains no frames.";
        }
    });

    function clampIndex(value: number): number {
        return Math.min(lastIndex, Math.max(0, value));
    }

    function requestScroll() {
        scrollRequest += 1;
    }

    function play() {
        if (frameCount < 2) {
            return;
        }
        if (atEnd) {
            frameIndex = 0;
        }
        playing = true;
    }

    function togglePlay() {
        if (playing) {
            playing = false;
        } else {
            play();
        }
    }

    function restart() {
        playing = false;
        frameIndex = 0;
        requestScroll();
    }

    function stepBy(delta: number) {
        playing = false;
        frameIndex = clampIndex(frameIndex + delta);
        requestScroll();
    }

    function jumpToJit() {
        if (firstJitIndex < 0) {
            return;
        }
        playing = false;
        frameIndex = firstJitIndex;
        requestScroll();
    }

    function handleSeek(event: Event) {
        const target = (event.currentTarget as HTMLInputElement).value;
        playing = false;
        frameIndex = clampIndex(Number(target));
        requestScroll();
    }

    function frameKindClasses(kind: ExecutionFrameKind | undefined): string {
        switch (kind) {
            case "step":
                return "bg-cyan-100 text-cyan-700";
            case "event":
                return "bg-violet-100 text-violet-700";
            case "jit":
                return "bg-amber-100 text-amber-700";
            case "boot":
                return "bg-slate-200 text-slate-600";
            case "end":
                return "bg-emerald-100 text-emerald-700";
            default:
                return "bg-slate-100 text-slate-500";
        }
    }

    function queueLabel(queue: string | undefined): string {
        if (queue === "micro_tasks") {
            return "microtask queue";
        }
        if (queue === "tasks") {
            return "task queue";
        }
        return "queue";
    }

    function narrationFor(frame: ExecutionFrame): string {
        const event = frame.event;

        if (event) {
            switch (event.type) {
                case "enqueue":
                    return `Enqueued ${event.item?.label ?? "callback"} in the ${queueLabel(event.queue)}.`;
                case "job_start":
                    return `Starting ${event.item?.label ?? "job"}${event.queue ? ` from the ${queueLabel(event.queue)}` : ""}.`;
                case "job_end":
                    return `Finished ${event.item?.label ?? "job"}.`;
                case "waiting":
                    return "Event loop is waiting for timers or async work.";
                case "timer_scheduled":
                    return event.timer
                        ? `Scheduled ${event.timer.label} (${event.timer.delay_ms}ms).`
                        : "Scheduled a timer.";
                case "timer_cancelled":
                    return event.timer
                        ? `Cancelled ${event.timer.label}.`
                        : "Cancelled a timer.";
                case "await_suspend":
                    return `${event.fn ?? "Async function"} suspended at await${event.resume_op_ptr !== undefined ? `; resumes at @${event.resume_op_ptr}` : ""}.`;
                case "jit_enter":
                    return `Entering ${event.fn ?? "function"} as compiled JIT code.`;
                case "jit_exit":
                    return `JIT returned from ${event.fn ?? "function"}; the result is on the stack.`;
                case "done":
                    return "Program finished.";
                case "error":
                    return event.detail ?? "Runtime error.";
            }
        }

        switch (frame.kind) {
            case "boot":
                return "Program compiled; the event loop is about to start.";
            case "step":
                return frame.vm
                    ? `${frame.vm.fn} executes ${frame.vm.op} at @${frame.vm.op_ptr}.`
                    : "Executing the next bytecode instruction.";
            case "jit":
                return frame.vm?.jit
                    ? `${frame.vm.jit.fn} is ${frame.vm.jit.state} in ${frame.vm.jit.mode} mode.`
                    : "Executing compiled JIT code.";
            case "end":
                return "Program finished.";
            default:
                return frame.note ?? frame.event?.detail ?? "Execution event.";
        }
    }

    function formatMs(value: number): string {
        return value < 10 ? value.toFixed(1) : value.toFixed(0);
    }

    $effect(() => {
        if (previousExecution === execution) {
            return;
        }

        previousExecution = execution;
        frameIndex = 0;
        playing = false;
    });

    $effect(() => {
        if (!playing || frameCount < 2) {
            return;
        }

        const interval = setInterval(
            () => {
                if (frameIndex >= frameCount - 1) {
                    playing = false;
                    return;
                }
                frameIndex += 1;
            },
            Math.max(16, 1000 / Math.max(1, speed)),
        );

        return () => clearInterval(interval);
    });

    $effect(() => {
        const frame = currentFrame;
        const vm = currentVm;
        const requested = scrollRequest;
        const shouldScroll = requested !== lastScrollRequest;
        lastScrollRequest = requested;

        if (frame === null || frame.kind === "boot" || frame.kind === "end") {
            setHighlight(null);
            return;
        }

        if (vm === null) {
            return;
        }

        setHighlight(
            {
                source: "op_code",
                astId: vm.ast_id,
                opCodePtr: vm.op_ptr,
                chunkId: vm.chunk_id,
            },
            { scroll: shouldScroll, toggle: false },
        );
    });

    $effect(() => {
        return () => setHighlight(null);
    });
</script>

<div class="flex h-full min-h-0 flex-col bg-white">
    {#if frameCount === 0}
        <div class="flex h-full items-center justify-center p-6">
            <div
                class="max-w-md rounded-xl border border-dashed border-slate-300 bg-slate-50 p-6 text-center"
            >
                <div
                    class="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-slate-200 text-lg text-slate-400"
                >
                    ⏱
                </div>
                <p class="mt-3 text-sm font-semibold text-slate-700">
                    No execution animation available
                </p>
                <p class="mt-1 text-xs leading-relaxed text-slate-500">
                    {unavailableMessage}
                </p>
                {#if execution}
                    <p class="mt-3 font-mono text-[10px] text-slate-400">
                        status: {execution.status} · frames: {execution.frames
                            .length}
                    </p>
                {/if}
                <p class="mt-3 text-[11px] text-slate-400">
                    The web runtime captures these frames with
                    <code class="rounded bg-white px-1 py-0.5 text-slate-500"
                        >--trace-execution</code
                    >.
                </p>
            </div>
        </div>
    {:else}
        <div class="shrink-0 border-b border-slate-100 px-2.5 py-2">
            <div class="flex flex-wrap items-center gap-1.5">
                <button
                    type="button"
                    onclick={togglePlay}
                    disabled={frameCount < 2}
                    class="inline-flex cursor-pointer items-center gap-1 rounded-md bg-[#00ADD8] px-2.5 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[#00758D] disabled:cursor-not-allowed disabled:opacity-50"
                >
                    {playing ? "❚❚ Pause" : "▶ Play"}
                </button>
                <button
                    type="button"
                    onclick={() => stepBy(-1)}
                    disabled={frameIndex <= 0}
                    title="Previous frame"
                    aria-label="Previous frame"
                    class="cursor-pointer rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-600 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
                >
                    ◀
                </button>
                <button
                    type="button"
                    onclick={() => stepBy(1)}
                    disabled={frameIndex >= lastIndex}
                    title="Next frame"
                    aria-label="Next frame"
                    class="cursor-pointer rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-600 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
                >
                    ▶
                </button>
                <button
                    type="button"
                    onclick={restart}
                    title="Restart"
                    aria-label="Restart animation"
                    class="cursor-pointer rounded-md border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-600 transition-colors hover:bg-slate-50"
                >
                    ↺
                </button>

                {#if firstJitIndex >= 0}
                    <button
                        type="button"
                        onclick={jumpToJit}
                        title="Jump to the first JIT frame"
                        class="inline-flex cursor-pointer items-center gap-1 rounded-md border border-amber-300 bg-amber-50 px-2 py-1.5 text-[10px] font-semibold text-amber-700 transition-colors hover:bg-amber-100"
                    >
                        JIT
                        <span
                            class="rounded-full bg-amber-500 px-1.5 py-0.5 text-[8px] font-bold text-white"
                        >
                            {jitFrameCount}
                        </span>
                    </button>
                {/if}

                <input
                    type="range"
                    min="0"
                    max={lastIndex}
                    step="1"
                    value={frameIndex}
                    disabled={frameCount < 2}
                    oninput={handleSeek}
                    aria-label="Execution frame"
                    class="min-w-24 flex-1 accent-cyan-600"
                />

                <span
                    class="shrink-0 rounded-md bg-slate-100 px-2 py-1 font-mono text-[10px] text-slate-600"
                >
                    {frameIndex + 1} / {frameCount}
                </span>

                <label
                    class="flex shrink-0 items-center gap-1 text-[10px] text-slate-500"
                >
                    Speed
                    <input
                        type="range"
                        min="1"
                        max="60"
                        step="1"
                        bind:value={speed}
                        aria-label="Playback speed"
                        class="w-16 accent-cyan-600"
                    />
                    <span class="w-10 font-mono text-[9px]">{speed} fps</span>
                </label>
            </div>

            <div
                class="mt-2 flex items-start gap-2 rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-1.5"
            >
                <span
                    class="shrink-0 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wide {frameKindClasses(
                        currentFrame?.kind,
                    )}"
                >
                    {currentFrame?.kind ?? "—"}
                </span>
                <div class="min-w-0 flex-1">
                    <p
                        class="truncate text-xs font-medium text-slate-700"
                        title={narration}
                    >
                        {narration}
                    </p>
                    <div
                        class="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-[9px] text-slate-400"
                    >
                        <span>frame {frameIndex + 1}/{frameCount}</span>
                        <span>seq {currentFrame?.seq ?? "—"}</span>
                        <span
                            >t+{formatMs(currentFrame?.at_ms ?? 0)}ms · run
                            {formatMs(execution?.duration_ms ?? 0)}ms</span
                        >
                        {#if currentFrame?.event}
                            <span
                                class="rounded bg-violet-100 px-1 py-0.5 font-semibold text-violet-600"
                            >
                                {currentFrame.event.type}
                            </span>
                        {/if}
                        {#if currentFrame?.vm}
                            <span class="truncate font-mono text-cyan-700"
                                >{currentFrame.vm.fn} · {currentFrame.vm
                                    .op}</span
                            >
                        {/if}
                        {#if currentFrame?.output?.delta}
                            <span class="text-emerald-600"
                                >+{currentFrame.output.delta.length} chars</span
                            >
                        {/if}
                    </div>
                    {#if secondaryNote}
                        <p class="mt-1 text-[10px] text-slate-500">
                            {secondaryNote}
                        </p>
                    {/if}
                </div>
            </div>

            {#if execution?.truncated}
                <div
                    class="mt-2 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-[10px] text-amber-700"
                    role="status"
                >
                    Trace truncated after {frameCount} stored frames.
                    {execution.dropped_frames} of {execution.total_frames}
                    attempted frames were dropped; the final output is still
                    shown.
                </div>
            {/if}
        </div>

        <div class="flex min-h-0 flex-1 flex-col gap-2 p-2">
            <div
                class="grid min-h-0 flex-[1.05] grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] gap-2"
            >
                <section
                    class="min-h-0 overflow-hidden rounded-lg border border-slate-200 bg-white"
                >
                    <ExecutionQueues
                        {queues}
                        {eventLoop}
                        frame={currentFrame}
                    />
                </section>

                <section
                    class="min-h-0 overflow-hidden rounded-lg border border-slate-200 bg-white"
                >
                    <ExecutionEventLoop
                        {eventLoop}
                        frame={currentFrame}
                        outputText={currentOutput}
                        {atEnd}
                    />
                </section>
            </div>

            <section
                class="flex min-h-0 flex-[0.95] flex-col overflow-hidden rounded-lg border border-slate-200 bg-white"
            >
                <ExecutionStack vm={currentVm} frame={currentFrame} />
            </section>
        </div>
    {/if}
</div>
