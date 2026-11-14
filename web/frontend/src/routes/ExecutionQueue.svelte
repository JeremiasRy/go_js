<script lang="ts">
    import { flip } from "svelte/animate";
    import type { QueueItem, QueueName } from "../types";

    type Props = {
        title: string;
        queueName: QueueName;
        items: QueueItem[];
        omitted?: number;
        active?: boolean;
        next?: boolean;
        flashId?: string | null;
    };

    const {
        title,
        queueName,
        items,
        omitted = 0,
        active = false,
        next = false,
        flashId = null,
    }: Props = $props();

    function sourceClasses(source: string | undefined): string {
        switch (source) {
            case "main":
                return "bg-slate-200 text-slate-600";
            case "setTimeout":
            case "setInterval":
                return "bg-amber-100 text-amber-700";
            case "await":
                return "bg-cyan-100 text-cyan-700";
            case "promise":
            case "queueMicrotask":
                return "bg-violet-100 text-violet-700";
            case "process.nextTick":
                return "bg-emerald-100 text-emerald-700";
            default:
                return "bg-slate-100 text-slate-500";
        }
    }

    function queueLabel(queue: QueueName): string {
        return queue === "micro_tasks" ? "microtask" : "task";
    }
</script>

<div
    class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border transition-colors {active
        ? 'border-cyan-300 bg-cyan-50/40'
        : 'border-slate-200 bg-slate-50/60'}"
    aria-label={`${title} queue`}
>
    <div
        class="flex shrink-0 items-center justify-between gap-2 border-b px-2.5 py-1.5 {active
            ? 'border-cyan-100'
            : 'border-slate-100'}"
    >
        <div class="flex min-w-0 items-center gap-1.5">
            <span
                class="h-1.5 w-1.5 shrink-0 rounded-full {active
                    ? 'animate-pulse bg-cyan-500'
                    : 'bg-slate-300'}"
            ></span>
            <span class="truncate text-[11px] font-semibold text-slate-600"
                >{title}</span
            >
            {#if next}
                <span
                    class="shrink-0 rounded-full bg-cyan-500 px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wide text-white"
                >
                    next
                </span>
            {/if}
        </div>
        <span class="shrink-0 text-[10px] text-slate-400">
            {items.length}{#if omitted > 0}+{omitted}{/if}
        </span>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-1.5">
        {#if items.length === 0}
            <div
                class="flex h-full min-h-8 items-center justify-center rounded-md border border-dashed border-slate-200 px-2 text-[10px] text-slate-300"
            >
                empty {queueLabel(queueName)} queue
            </div>
        {:else}
            <div class="flex flex-col gap-1">
                {#each items as item, index (item.id)}
                    <div
                        animate:flip={{ duration: 180 }}
                        class="rounded-md border px-2 py-1.5 transition-all {index ===
                        0 && next
                            ? 'border-cyan-300 bg-white shadow-sm shadow-cyan-100'
                            : 'border-slate-200 bg-white'} {item.id === flashId
                            ? 'queue-flash'
                            : ''}"
                    >
                        <div class="flex min-w-0 items-center gap-1.5">
                            <span
                                class="shrink-0 rounded px-1 py-0.5 font-mono text-[9px] font-semibold {index ===
                                0 && next
                                    ? 'bg-cyan-500 text-white'
                                    : 'bg-slate-100 text-slate-400'}"
                            >
                                {index === 0 && next ? "RUN" : index + 1}
                            </span>
                            <span
                                class="min-w-0 truncate text-[11px] font-medium text-slate-700"
                                title={item.label}
                            >
                                {item.label}
                            </span>
                        </div>
                        <div class="mt-1 flex min-w-0 flex-wrap items-center gap-1">
                            {#if item.source}
                                <span
                                    class="rounded px-1 py-0.5 text-[9px] font-medium {sourceClasses(
                                        item.source,
                                    )}"
                                >
                                    {item.source}
                                </span>
                            {/if}
                            {#if item.fn}
                                <span
                                    class="max-w-28 truncate rounded bg-white px-1 py-0.5 font-mono text-[9px] text-slate-400 ring-1 ring-slate-100"
                                >
                                    {item.fn}
                                </span>
                            {/if}
                            {#if item.native}
                                <span
                                    class="rounded bg-slate-100 px-1 py-0.5 text-[9px] text-slate-400"
                                >
                                    native
                                </span>
                            {/if}
                            <span class="ml-auto font-mono text-[9px] text-slate-300"
                                >{item.id}</span
                            >
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    {#if omitted > 0}
        <div
            class="shrink-0 border-t border-slate-100 px-2.5 py-1 text-[9px] text-slate-400"
        >
            +{omitted} more {queueLabel(queueName)}
            {omitted === 1 ? "" : "s"} hidden by the trace limit
        </div>
    {/if}
</div>

<style>
    .queue-flash {
        animation: queue-flash 500ms ease-out;
    }

    @keyframes queue-flash {
        0% {
            box-shadow: 0 0 0 2px rgb(6 182 212 / 55%);
            transform: translateY(-2px);
        }
        100% {
            box-shadow: 0 0 0 0 rgb(6 182 212 / 0%);
            transform: translateY(0);
        }
    }
</style>
