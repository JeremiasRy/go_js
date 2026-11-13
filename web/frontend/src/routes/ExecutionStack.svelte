<script lang="ts">
    import { fly } from "svelte/transition";
    import type {
        ExecutionFrame,
        ValueView,
        VMStackFrame,
        VMState,
    } from "../types";
    import ExecutionValue from "./ExecutionValue.svelte";

    type Props = {
        vm: VMState | null;
        frame: ExecutionFrame | null;
    };

    type Slot = {
        abs: number;
        value: ValueView;
        owner: VMStackFrame | null;
    };

    type FrameStyle = {
        bar: string;
        dot: string;
        text: string;
        chip: string;
    };

    const frameStyles: FrameStyle[] = [
        {
            bar: "border-b-cyan-500",
            dot: "bg-cyan-500",
            text: "text-cyan-700",
            chip: "bg-cyan-50 text-cyan-700",
        },
        {
            bar: "border-b-rose-500",
            dot: "bg-rose-500",
            text: "text-rose-700",
            chip: "bg-rose-50 text-rose-700",
        },
        {
            bar: "border-b-amber-500",
            dot: "bg-amber-500",
            text: "text-amber-700",
            chip: "bg-amber-50 text-amber-700",
        },
        {
            bar: "border-b-violet-500",
            dot: "bg-violet-500",
            text: "text-violet-700",
            chip: "bg-violet-50 text-violet-700",
        },
        {
            bar: "border-b-emerald-500",
            dot: "bg-emerald-500",
            text: "text-emerald-700",
            chip: "bg-emerald-50 text-emerald-700",
        },
        {
            bar: "border-b-blue-500",
            dot: "bg-blue-500",
            text: "text-blue-700",
            chip: "bg-blue-50 text-blue-700",
        },
        {
            bar: "border-b-fuchsia-500",
            dot: "bg-fuchsia-500",
            text: "text-fuchsia-700",
            chip: "bg-fuchsia-50 text-fuchsia-700",
        },
        {
            bar: "border-b-orange-500",
            dot: "bg-orange-500",
            text: "text-orange-700",
            chip: "bg-orange-50 text-orange-700",
        },
    ];

    const { vm, frame }: Props = $props();

    const activeFrame = $derived.by<VMStackFrame | null>(() => {
        const state = vm;
        if (state === null || state.frames.length === 0) {
            return null;
        }

        const index = Math.min(
            Math.max(state.active_frame, 0),
            state.frames.length - 1,
        );
        return state.frames[index] ?? null;
    });

    const totalDepth = $derived(
        vm === null ? 0 : vm.frames.length + (vm.frames_omitted ?? 0),
    );

    const activeDepth = $derived(
        vm === null || activeFrame === null
            ? 0
            : (vm.frames_omitted ?? 0) + activeFrame.index + 1,
    );

    const slots = $derived.by<Slot[]>(() => {
        const state = vm;
        if (state === null) {
            return [];
        }

        const capturedStart = state.stack_base;
        const capturedEnd = state.stack_base + state.stack.length;
        const end = Math.min(state.stack_top, capturedEnd);
        const result: Slot[] = [];

        for (let abs = capturedStart; abs < end; abs += 1) {
            const value = state.stack[abs - capturedStart];
            if (value === undefined) {
                continue;
            }

            let owner: VMStackFrame | null = state.frames[0] ?? null;
            for (const stackFrame of state.frames) {
                if (stackFrame.local_start <= abs) {
                    owner = stackFrame;
                } else {
                    break;
                }
            }

            result.push({ abs, value, owner });
        }

        return result;
    });

    const hiddenSlots = $derived(
        vm === null ? 0 : Math.max(vm.stack_base, vm.stack_omitted ?? 0),
    );

    const topAbsolute = $derived(
        vm === null
            ? -1
            : Math.min(
                  vm.stack_top - 1,
                  vm.stack_base + vm.stack.length - 1,
              ),
    );

    const panelTitle = $derived(
        frame?.kind === "jit" ? "JIT stack" : "VM stack",
    );

    function styleFor(index: number | undefined): FrameStyle {
        return frameStyles[Math.abs(index ?? 0) % frameStyles.length]!;
    }

    function formatDuration(value: number | undefined): string {
        if (value === undefined) {
            return "";
        }
        return `${value < 10 ? value.toFixed(2) : value.toFixed(1)}ms`;
    }
</script>

<div class="flex h-full min-h-0 flex-col">
    <div
        class="flex shrink-0 items-center justify-between gap-2 border-b border-slate-100 px-3 py-2"
    >
        <div class="flex min-w-0 items-center gap-2">
            <span class="text-xs font-semibold text-slate-600"
                >{panelTitle}</span
            >
            <span class="text-[10px] text-slate-400">left → right</span>
            {#if vm}
                <span
                    class="rounded-full bg-slate-100 px-1.5 py-0.5 text-[9px] font-semibold text-slate-500"
                >
                    vm {vm.id}
                </span>
                {#if vm.parent_id}
                    <span
                        class="rounded-full bg-indigo-50 px-1.5 py-0.5 text-[9px] font-semibold text-indigo-500"
                    >
                        sub-VM of {vm.parent_id}
                    </span>
                {/if}
            {/if}
        </div>
        {#if vm}
            <div
                class="flex shrink-0 items-center gap-2 text-[10px] text-slate-400"
            >
                <span class="font-semibold text-slate-500"
                    >depth {activeDepth} / {totalDepth}</span
                >
                <span>{slots.length} slots</span>
            </div>
        {/if}
    </div>

    <div class="min-h-0 flex-1 overflow-hidden p-2">
        {#if vm === null}
            <div
                class="flex h-full items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50 px-4 text-center text-xs text-slate-400"
            >
                No VM state yet. The stack appears when the first job starts.
            </div>
        {:else}
            <div
                class="flex h-full min-h-0 flex-col overflow-hidden rounded-lg border border-slate-200 bg-white"
            >
                <header
                    class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-slate-100 bg-slate-50/70 px-2.5 py-2"
                >
                    <div class="flex min-w-0 items-center gap-1.5">
                        {#if activeFrame}
                            <span
                                class="rounded bg-cyan-500 px-1.5 py-0.5 font-mono text-[9px] font-semibold text-white"
                            >
                                #{activeFrame.index}
                            </span>
                            <span
                                class="min-w-0 truncate text-sm font-semibold text-slate-700"
                                title={activeFrame.fn}
                            >
                                {activeFrame.fn}
                            </span>
                            <span
                                class="shrink-0 rounded-full bg-cyan-500 px-1.5 py-0.5 text-[9px] font-semibold text-white"
                            >
                                active
                            </span>
                            <span
                                class="min-w-0 truncate font-mono text-[10px] text-cyan-700"
                            >
                                {activeFrame.op} @{activeFrame.op_ptr}
                            </span>
                            <span
                                class="shrink-0 rounded bg-white px-1 py-0.5 font-mono text-[9px] text-slate-400"
                            >
                                {activeFrame.chunk_id}
                            </span>
                        {:else}
                            <span class="text-xs text-slate-400"
                                >No active call frame.</span
                            >
                        {/if}
                    </div>
                    {#if activeFrame}
                        <span
                            class="shrink-0 rounded-full bg-white px-1.5 py-0.5 text-[9px] font-semibold text-slate-500 ring-1 ring-slate-200"
                        >
                            call depth {activeDepth} / {totalDepth}
                        </span>
                    {/if}
                </header>

                {#if vm.jit}
                    <div
                        class="flex shrink-0 flex-wrap items-center gap-2 border-b border-amber-100 bg-amber-50 px-2.5 py-1.5 text-[10px] text-amber-700"
                    >
                        <span
                            class="rounded bg-amber-500 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wide text-white"
                        >
                            JIT {vm.jit.state}
                        </span>
                        <span class="font-semibold">{vm.jit.fn}</span>
                        <span>{vm.jit.mode} mode</span>
                        {#if vm.jit.result}
                            <span class="font-mono"
                                >→ {vm.jit.result.display}</span
                            >
                        {/if}
                        {#if vm.jit.duration_ms !== undefined}
                            <span class="ml-auto font-mono text-[9px]"
                                >{formatDuration(vm.jit.duration_ms)}</span
                            >
                        {/if}
                    </div>
                {/if}

                <div
                    class="flex min-h-0 flex-1 items-stretch gap-2 overflow-x-auto p-2"
                >
                    {#if hiddenSlots > 0}
                        <div
                            class="flex w-24 shrink-0 flex-col items-center justify-center rounded-lg border border-dashed border-slate-300 bg-slate-50 p-2 text-center"
                        >
                            <span class="text-lg text-slate-300">…</span>
                            <span class="text-[10px] text-slate-400">
                                {hiddenSlots} hidden slot{hiddenSlots === 1
                                    ? ""
                                    : "s"}
                            </span>
                        </div>
                    {/if}

                    {#each slots as slot (slot.abs)}
                        {@const style = styleFor(slot.owner?.index)}
                        {@const active = slot.owner?.index ===
                            activeFrame?.index}
                        <div
                            in:fly={{ x: -10, duration: 140 }}
                            class="flex w-32 shrink-0 flex-col rounded-md border-b-4 {style.bar} pb-1 {active
                                ? 'bg-slate-50'
                                : ''}"
                        >
                            <div
                                class="flex min-w-0 items-center gap-1 px-1 pt-1"
                            >
                                <span
                                    class="h-1.5 w-1.5 shrink-0 rounded-full {style.dot}"
                                ></span>
                                <span
                                    class="truncate font-mono text-[9px] text-slate-400"
                                >
                                    slot {slot.abs}
                                </span>
                            </div>
                            <ExecutionValue
                                value={slot.value}
                                top={slot.abs === topAbsolute}
                                compact
                            />
                            <div
                                class="mt-1 flex min-w-0 items-center gap-1 px-1"
                            >
                                <span
                                    class="truncate text-[9px] font-medium {style.text}"
                                    title={slot.owner?.fn ?? "unknown frame"}
                                >
                                    {slot.owner
                                        ? `#${slot.owner.index} ${slot.owner.fn}`
                                        : "unknown frame"}
                                </span>
                                {#if active}
                                    <span
                                        class="ml-auto shrink-0 rounded px-1 py-0.5 text-[8px] font-semibold {style.chip}"
                                    >
                                        active
                                    </span>
                                {/if}
                            </div>
                        </div>
                    {/each}

                    {#if slots.length === 0 && hiddenSlots === 0}
                        <div
                            class="flex min-w-40 flex-1 items-center justify-center rounded-lg border border-dashed border-slate-200 px-3 py-3 text-[10px] text-slate-300"
                        >
                            empty stack
                        </div>
                    {/if}
                </div>

                <footer
                    class="flex shrink-0 items-center justify-between gap-2 border-t border-slate-100 px-2.5 py-1 text-[9px] text-slate-400"
                >
                    <span>stack_base {vm.stack_base}</span>
                    <span>stack_top {vm.stack_top}</span>
                    {#if vm.frames_omitted}
                        <span>{vm.frames_omitted} callers hidden</span>
                    {/if}
                </footer>
            </div>
        {/if}
    </div>
</div>
