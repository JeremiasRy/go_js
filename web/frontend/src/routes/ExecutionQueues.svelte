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
        frame?.event?.type === "enqueue"
            ? (frame.event.item?.id ?? null)
            : null,
    );

    const flashQueue = $derived(
        frame?.event?.type === "enqueue" ? (frame.event.queue ?? null) : null,
    );
</script>

<div class="flex h-full min-h-0 flex-col gap-y-2">
    <ExecutionQueue
        title="Microtasks"
        queueName="micro_tasks"
        items={queues.micro_tasks}
        omitted={queues.micro_tasks_omitted ?? 0}
        next={nextQueue === "micro_tasks"}
        flashId={flashQueue === "micro_tasks" ? flashId : null}
    />
    <ExecutionQueue
        title="Tasks"
        queueName="tasks"
        items={queues.tasks}
        omitted={queues.tasks_omitted ?? 0}
        next={nextQueue === "tasks"}
        flashId={flashQueue === "tasks" ? flashId : null}
    />
</div>
