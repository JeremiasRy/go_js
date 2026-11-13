<script lang="ts">
    import type { ExecutionFrame, EventLoopState, TimerItem } from "../types";

    type Props = {
        eventLoop: EventLoopState;
        frame: ExecutionFrame | null;
        outputText: string;
        atEnd: boolean;
    };

    const { eventLoop, frame, outputText, atEnd }: Props = $props();

    let outputElement = $state<HTMLPreElement | undefined>(undefined);
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

    const timerFlash = $derived(
        frame?.event?.type === "timer_scheduled" ||
            frame?.event?.type === "timer_cancelled",
    );

    function timerClasses(timer: TimerItem): string {
        switch (timer.state) {
            case "pending":
                return "border-amber-200 bg-amber-50 text-amber-700";
            case "fired":
                return "border-emerald-200 bg-emerald-50 text-emerald-700";
            case "cancelled":
                return "border-red-200 bg-red-50 text-red-600";
            default:
                return "border-slate-200 bg-slate-50 text-slate-500";
        }
    }

    $effect(() => {
        const length = outputText.length;
        if (outputElement === undefined || length === lastOutputLength) {
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
                    <p class="text-slate-400">next queue</p>
                    <p class="truncate font-semibold text-slate-700">
                        {eventLoop.next_queue ?? "—"}
                    </p>
                </div>
                <div class="rounded-md bg-slate-50 px-2 py-1">
                    <p class="text-slate-400">outstanding work</p>
                    <p class="truncate font-semibold text-slate-700">
                        {eventLoop.work}
                    </p>
                </div>
                <div class="rounded-md bg-slate-50 px-2 py-1">
                    <p class="text-slate-400">timers</p>
                    <p class="truncate font-semibold text-slate-700">
                        {eventLoop.timers.length +
                            (eventLoop.timers_omitted ?? 0)}
                    </p>
                </div>
            </div>
        </div>

        <div
            class="shrink-0 rounded-lg border p-2 transition-all {timerFlash
                ? 'timer-flash border-amber-300'
                : 'border-slate-200'} bg-white"
        >
            <div class="flex items-center justify-between gap-2">
                <span class="text-[10px] font-semibold uppercase tracking-wide text-slate-400">
                    Timers
                </span>
                <span class="text-[9px] text-slate-400">
                    {eventLoop.timers.length +
                        (eventLoop.timers_omitted ?? 0)} tracked
                </span>
            </div>
            {#if eventLoop.timers.length === 0}
                <p class="mt-1 text-xs text-slate-400">No timers scheduled.</p>
            {:else}
                <ul class="mt-1 flex flex-col gap-1">
                    {#each eventLoop.timers as timer (timer.id)}
                        <li
                            class="flex min-w-0 items-center gap-1.5 rounded-md border px-1.5 py-1 {timerClasses(
                                timer,
                            )}"
                        >
                            <span class="shrink-0 font-mono text-[9px]"
                                >#{timer.id}</span
                            >
                            <span class="min-w-0 flex-1 truncate text-[10px] font-medium">
                                {timer.label}
                            </span>
                            <span class="shrink-0 text-[9px]">{timer.delay_ms}ms</span>
                            <span
                                class="shrink-0 rounded bg-white/70 px-1 py-0.5 text-[8px] font-semibold uppercase"
                            >
                                {timer.state}
                            </span>
                        </li>
                    {/each}
                </ul>
                {#if eventLoop.timers_omitted}
                    <p class="mt-1 text-[9px] text-slate-400">
                        +{eventLoop.timers_omitted} more hidden by the trace limit
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
                <span class="text-[10px] font-semibold uppercase tracking-wide text-slate-400">
                    Output
                </span>
                <span class="text-[9px] text-slate-500">
                    {atEnd ? "final" : `${outputText.length} chars`}
                </span>
            </div>
            <pre
                bind:this={outputElement}
                class="min-h-0 flex-1 overflow-y-auto whitespace-pre-wrap break-words p-2 font-mono text-[11px] leading-relaxed text-slate-100"
            >{outputText || "(no output yet)"}</pre>
        </div>
    </div>
</div>

<style>
    .timer-flash {
        animation: timer-flash 450ms ease-out;
    }

    @keyframes timer-flash {
        0% {
            box-shadow: 0 0 0 2px rgb(245 158 11 / 45%);
        }
        100% {
            box-shadow: 0 0 0 0 rgb(245 158 11 / 0%);
        }
    }
</style>
