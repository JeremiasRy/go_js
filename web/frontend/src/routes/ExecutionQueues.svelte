<script lang="ts">
    import type { ExecutionFrame, EventLoopState, QueueState } from "../types";
    import ExecutionQueue from "./ExecutionQueue.svelte";

    type Props = {
        queues: QueueState;
        eventLoop: EventLoopState;
        frame: ExecutionFrame | null;
    };

    const { queues, eventLoop, frame }: Props = $props();

    const nextQueue = $derived(
        eventLoop.next_queue ??
            (queues.micro_tasks.length > 0
                ? "micro_tasks"
                : queues.tasks.length > 0
                  ? "tasks"
                  : null),
    );

    const activeQueue = $derived(
        eventLoop.current_job?.kind === "micro_task"
            ? "micro_tasks"
            : eventLoop.current_job?.kind === "task"
              ? "tasks"
              : nextQueue,
    );

    const flashId = $derived(
        frame?.event?.type === "enqueue" ? (frame.event.item?.id ?? null) : null,
    );

    const flashQueue = $derived(
        frame?.event?.type === "enqueue" ? (frame.event.queue ?? null) : null,
    );
</script>

<div class="flex h-full min-h-0 flex-col">
    <div
        class="flex shrink-0 items-center justify-between gap-2 border-b border-slate-100 px-3 py-2"
    >
        <div class="flex items-center gap-2">
            <span class="text-xs font-semibold text-slate-600">Task queues</span>
            <span class="text-[10px] text-slate-400">front → back</span>
        </div>
        <span class="text-[10px] text-slate-400">
            microtasks drain first
        </span>
    </div>

    <div class="grid min-h-0 flex-1 grid-rows-2 gap-2 p-2">
        <ExecutionQueue
            title="Microtasks"
            queueName="micro_tasks"
            items={queues.micro_tasks}
            omitted={queues.micro_tasks_omitted ?? 0}
            active={activeQueue === "micro_tasks"}
            next={nextQueue === "micro_tasks"}
            flashId={flashQueue === "micro_tasks" ? flashId : null}
        />
        <ExecutionQueue
            title="Tasks"
            queueName="tasks"
            items={queues.tasks}
            omitted={queues.tasks_omitted ?? 0}
            active={activeQueue === "tasks"}
            next={nextQueue === "tasks"}
            flashId={flashQueue === "tasks" ? flashId : null}
        />
    </div>
</div>
