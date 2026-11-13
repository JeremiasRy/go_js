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
    };

    type FrameGroup = {
        frame: VMStackFrame;
        hidden: number;
        slots: Slot[];
        active: boolean;
    };

    const { vm, frame }: Props = $props();

    const groups = $derived.by<FrameGroup[]>(() => {
        const state = vm;
        if (state === null) {
            return [];
        }

        const capturedStart = state.stack_base;
        const capturedEnd = state.stack_base + state.stack.length;

        return state.frames.map((stackFrame, index) => {
            const nextStart =
                state.frames[index + 1]?.local_start ?? state.stack_top;
            const start = Math.max(stackFrame.local_start, capturedStart);
            const end = Math.min(nextStart, capturedEnd);
            const slots: Slot[] = [];

            for (let abs = start; abs < end; abs += 1) {
                const value = state.stack[abs - capturedStart];
                if (value !== undefined) {
                    slots.push({ abs, value });
                }
            }

            return {
                frame: stackFrame,
                hidden: Math.max(0, capturedStart - stackFrame.local_start),
                slots,
                active: stackFrame.index === state.active_frame,
            };
        });
    });

    const topAbsolute = $derived(
        vm === null ? -1 : Math.min(vm.stack_top - 1, vm.stack_base + vm.stack.length - 1),
    );

    const frameLabel = $derived(frame?.kind === "jit" ? "JIT" : "VM stack");

    function slotLabel(count: number): string {
        return `${count} slot${count === 1 ? "" : "s"}`;
    }
</script>

<div class="flex h-full min-h-0 flex-col">
    <div
        class="flex shrink-0 items-center justify-between gap-2 border-b border-slate-100 px-3 py-2"
    >
        <div class="flex min-w-0 items-center gap-2">
            <span class="text-xs font-semibold text-slate-600">{frameLabel}</span>
            <span class="text-[10px] text-slate-400">bottom → top</span>
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
                {#if vm.jit}
                    <span
                        class="rounded-full px-1.5 py-0.5 text-[9px] font-semibold {vm.jit.state ===
                        'running'
                            ? 'bg-amber-100 text-amber-700'
                            : 'bg-slate-100 text-slate-500'}"
                    >
                        {vm.jit.mode} · {vm.jit.state}
                    </span>
                {/if}
            {/if}
        </div>
        {#if vm}
            <div class="flex shrink-0 items-center gap-2 text-[10px] text-slate-400">
                {#if vm.frames_omitted}
                    <span>{vm.frames_omitted} frames hidden</span>
                {/if}
                {#if vm.stack_omitted}
                    <span>{vm.stack_omitted} slots hidden</span>
                {/if}
                <span>{slotLabel(vm.stack.length)} shown</span>
            </div>
        {/if}
    </div>

    <div class="min-h-0 flex-1 overflow-x-auto overflow-y-hidden p-3">
        {#if vm === null}
            <div
                class="flex h-full items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50 px-4 text-center text-xs text-slate-400"
            >
                No VM state yet. The stack appears when the first job starts.
            </div>
        {:else}
            <div class="flex h-full min-w-max items-stretch gap-2">
                {#if vm.frames_omitted}
                    <div
                        class="flex w-24 shrink-0 flex-col items-center justify-center rounded-lg border border-dashed border-slate-300 bg-slate-50 p-2 text-center"
                    >
                        <span class="text-lg text-slate-300">…</span>
                        <span class="text-[10px] text-slate-400">
                            {vm.frames_omitted} hidden
                            frame{vm.frames_omitted === 1 ? "" : "s"}
                        </span>
                    </div>
                {/if}

                {#each groups as group (group.frame.index)}
                    <section
                        in:fly={{ x: -10, duration: 180 }}
                        class="flex w-56 shrink-0 flex-col overflow-hidden rounded-lg border bg-white xl:w-64 {group.active
                            ? 'border-cyan-300 shadow-sm shadow-cyan-100'
                            : 'border-slate-200'}"
                    >
                        <header
                            class="shrink-0 border-b px-2.5 py-2 {group.active
                                ? 'border-cyan-100 bg-cyan-50/70'
                                : 'border-slate-100 bg-slate-50/70'}"
                        >
                            <div class="flex min-w-0 items-center gap-1.5">
                                <span
                                    class="rounded bg-slate-200 px-1 py-0.5 font-mono text-[9px] font-semibold text-slate-500"
                                >
                                    #{group.frame.index}
                                </span>
                                <span
                                    class="min-w-0 truncate text-xs font-semibold text-slate-700"
                                    title={group.frame.fn}
                                >
                                    {group.frame.fn}
                                </span>
                                {#if group.active}
                                    <span
                                        class="ml-auto shrink-0 rounded-full bg-cyan-500 px-1.5 py-0.5 text-[9px] font-semibold text-white"
                                    >
                                        active
                                    </span>
                                {/if}
                            </div>
                            <div
                                class="mt-1 flex min-w-0 items-center gap-1.5 text-[10px] text-slate-400"
                            >
                                <span class="truncate font-mono text-cyan-700"
                                    >{group.frame.op}</span
                                >
                                <span>@{group.frame.op_ptr}</span>
                                <span class="ml-auto shrink-0 rounded bg-white px-1 py-0.5 font-mono text-[9px] text-slate-400">
                                    {group.frame.chunk_id}
                                </span>
                            </div>
                        </header>

                        <div class="flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto p-2">
                            {#if group.hidden > 0}
                                <div
                                    class="rounded-md border border-dashed border-slate-300 bg-slate-50 px-2 py-1 text-center text-[10px] text-slate-400"
                                >
                                    … {group.hidden} hidden slot{group.hidden === 1
                                        ? ""
                                        : "s"}
                                </div>
                            {/if}

                            {#each group.slots as slot (slot.abs)}
                                <div in:fly={{ y: 4, duration: 140 }}>
                                    <ExecutionValue
                                        value={slot.value}
                                        top={slot.abs === topAbsolute}
                                        compact
                                    />
                                </div>
                            {/each}

                            {#if group.slots.length === 0 && group.hidden === 0}
                                <div
                                    class="flex flex-1 items-center justify-center rounded-md border border-dashed border-slate-200 px-2 py-3 text-[10px] text-slate-300"
                                >
                                    empty frame
                                </div>
                            {/if}
                        </div>

                        <footer
                            class="shrink-0 border-t border-slate-100 px-2.5 py-1 text-[9px] text-slate-400"
                        >
                            local_start {group.frame.local_start}
                        </footer>
                    </section>
                {/each}
            </div>
        {/if}
    </div>
</div>
