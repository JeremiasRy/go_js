<script lang="ts">
    import type { ExecutionFrame, EventLoopState, WorkItem } from "../types";

    type Props = {
        eventLoop: EventLoopState;
        frame: ExecutionFrame | null;
        outputText: string;
        atEnd: boolean;
    };

    const { eventLoop, frame, outputText, atEnd }: Props = $props();

    let outputElement = $state<HTMLPreElement | null>(null);
    let lastOutputLength = -1;

    const phaseClasses = $derived.by(() => {
        switch (eventLoop.phase) {
            case "executing":
                return {
                    badge: "bg-cyan-100 text-cyan-700",
                    ring: "border-cyan-400",
                    glyph: "▶",
                };
            case "queued":
                return {
                    badge: "bg-amber-100 text-amber-700",
                    ring: "border-amber-400",
                    glyph: "≡",
                };
            case "waiting":
                return {
                    badge: "bg-violet-100 text-violet-700",
                    ring: "border-violet-400",
                    glyph: "⏳",
                };
            case "idle":
                return {
                    badge: "bg-slate-100 text-slate-500",
                    ring: "border-slate-300",
                    glyph: "·",
                };
            case "done":
                return {
                    badge: "bg-emerald-100 text-emerald-700",
                    ring: "border-emerald-400",
                    glyph: "✓",
                };
            default:
                return {
                    badge: "bg-slate-100 text-slate-500",
                    ring: "border-slate-300",
                    glyph: "…",
                };
        }
    });

    const phaseLabel = $derived(
        eventLoop.phase === "executing" && eventLoop.current_job
            ? `executing ${eventLoop.current_job.kind === "micro_task" ? "microtask" : "task"}`
            : eventLoop.phase,
    );

    const workItems = $derived(eventLoop.work_items ?? []);
    const workFlash = $derived(
        frame?.event?.type === "timer_scheduled" ||
            frame?.event?.type === "timer_cancelled" ||
            frame?.event?.type === "job_start" ||
            frame?.event?.type === "job_end" ||
            frame?.event?.type === "enqueue",
    );

    function workClasses(item: WorkItem): string {
        switch (item.kind) {
            case "timer":
                return "border-amber-200 bg-amber-50 text-amber-700";
            case "async":
                return "border-cyan-200 bg-cyan-50 text-cyan-700";
            case "fetch":
                return "border-violet-200 bg-violet-50 text-violet-700";
            case "main":
                return "border-emerald-200 bg-emerald-50 text-emerald-700";
            default:
                return "border-slate-200 bg-slate-50 text-slate-500";
        }
    }

    $effect(() => {
        const length = outputText.length;
        if (!outputElement || length === lastOutputLength) {
            return;
        }

        lastOutputLength = length;
        outputElement.scrollTop = outputElement.scrollHeight;
    });
</script>

<div class="flex h-full min-h-0 flex-col">
    <div
        class="flex shrink-0 items-center justify-between gap-2 border-b border-slate-100 px-3 py-2"
    >
        <div class="flex items-center gap-2">
            <span class="text-xs font-semibold text-slate-600">Event loop</span>
            <span
                class="rounded-full px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide {phaseClasses.badge}"
            >
                {phaseLabel}
            </span>
        </div>
        <span class="text-[10px] text-slate-400">
            work {eventLoop.work}
        </span>
    </div>

    <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-2">
        <div
            class="flex shrink-0 items-center gap-3 rounded-lg border border-slate-200 bg-white p-2"
        >
            <div
                class="relative flex h-16 w-16 shrink-0 items-center justify-center rounded-full border-4 {phaseClasses.ring}"
            >
                <span class="text-lg text-slate-500">{phaseClasses.glyph}</span>
                <span
                    class="absolute -bottom-1 -right-1 rounded-full border border-white bg-slate-700 px-1.5 py-0.5 text-[8px] font-semibold uppercase text-white"
                >
                    loop
                </span>
            </div>
            <div class="grid min-w-0 flex-1 grid-cols-2 gap-1.5 text-[10px]">
                <div class="rounded-md bg-slate-50 px-2 py-1">
                    <p class="text-slate-400">phase</p>
                    <p class="truncate font-semibold text-slate-700">
                        {phaseLabel}
                    </p>
                </div>
                <div class="rounded-md bg-slate-50 px-2 py-1">
                    <p class="text-slate-400">work</p>
                    <p class="truncate font-semibold text-slate-700">
                        {eventLoop.work}
                    </p>
                </div>
            </div>
        </div>

        <div
            class="shrink-0 rounded-lg border p-2 transition-all {workFlash
                ? 'work-flash border-amber-300'
                : 'border-slate-200'} bg-white"
        >
            <div class="flex items-center justify-between gap-2">
                <span
                    class="text-[10px] font-semibold uppercase tracking-wide text-slate-400"
                >
                    Work
                </span>
                <span class="text-[9px] text-slate-400">
                    {eventLoop.work} outstanding
                </span>
            </div>
            {#if workItems.length === 0}
                <p class="mt-1 text-xs text-slate-400">
                    No background work running.
                </p>
            {:else}
                <ul class="mt-1 flex flex-col gap-1">
                    {#each workItems as item (item.id)}
                        <li
                            class="flex min-w-0 items-center gap-1.5 rounded-md border px-1.5 py-1 {workClasses(
                                item,
                            )}"
                        >
                            <span
                                class="shrink-0 rounded bg-white/70 px-1 py-0.5 text-[8px] font-semibold uppercase"
                            >
                                {item.kind}
                            </span>
                            <span
                                class="min-w-0 flex-1 truncate text-[10px] font-medium"
                                title={item.label}
                            >
                                {item.label}
                            </span>
                            {#if item.delay_ms !== undefined}
                                <span class="shrink-0 text-[9px]"
                                    >{item.delay_ms}ms</span
                                >
                            {/if}
                            {#if item.repeats}
                                <span
                                    class="shrink-0 rounded bg-white/70 px-1 py-0.5 text-[8px] font-semibold uppercase"
                                >
                                    repeat
                                </span>
                            {/if}
                        </li>
                    {/each}
                </ul>
                {#if eventLoop.work_omitted}
                    <p class="mt-1 text-[9px] text-slate-400">
                        +{eventLoop.work_omitted} more hidden by the trace limit
                    </p>
                {/if}
            {/if}
        </div>

        <div
            class="flex min-h-24 flex-1 flex-col overflow-hidden rounded-lg border border-slate-700 bg-slate-900"
        >
            <div
                class="flex shrink-0 items-center justify-between gap-2 border-b border-slate-700 px-2 py-1"
            >
                <span
                    class="text-[10px] font-semibold uppercase tracking-wide text-slate-400"
                >
                    Output
                </span>
                <span class="text-[9px] text-slate-500">
                    {atEnd ? "final" : `${outputText.length} chars`}
                </span>
            </div>
            <pre
                bind:this={outputElement}
                class="min-h-0 flex-1 overflow-y-auto whitespace-pre-wrap break-words p-2 font-mono text-[11px] leading-relaxed text-slate-100">{outputText ||
                    "(no output yet)"}</pre>
        </div>
    </div>
</div>

<style>
    .work-flash {
        animation: work-flash 450ms ease-out;
    }

    @keyframes work-flash {
        0% {
            box-shadow: 0 0 0 2px rgb(245 158 11 / 45%);
        }
        100% {
            box-shadow: 0 0 0 0 rgb(245 158 11 / 0%);
        }
    }
</style>
