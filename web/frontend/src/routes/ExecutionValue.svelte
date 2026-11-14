<script lang="ts">
    import type { ValueView } from "../types";

    type Props = {
        value: ValueView;
        top?: boolean;
        compact?: boolean;
    };

    const { value, top = false, compact = false }: Props = $props();

    const synthetic = $derived(
        value.synthetic === true || value.type === "note",
    );

    function typeClasses(type: string, isSynthetic: boolean): string {
        if (isSynthetic) {
            return "border-amber-300 bg-amber-100 text-amber-700";
        }

        switch (type) {
            case "number":
                return "border-cyan-200 bg-cyan-50 text-cyan-700";
            case "string":
                return "border-emerald-200 bg-emerald-50 text-emerald-700";
            case "boolean":
                return "border-violet-200 bg-violet-50 text-violet-700";
            case "function":
            case "method_handle":
                return "border-fuchsia-200 bg-fuchsia-50 text-fuchsia-700";
            case "promise":
                return "border-blue-200 bg-blue-50 text-blue-700";
            case "error":
                return "border-red-200 bg-red-50 text-red-700";
            case "array":
            case "object":
            case "map":
            case "set":
            case "regexp":
            case "class":
                return "border-slate-200 bg-slate-100 text-slate-600";
            case "null":
            case "undefined":
            case "empty":
                return "border-slate-200 bg-slate-50 text-slate-400";
            default:
                return "border-slate-200 bg-slate-100 text-slate-600";
        }
    }

    function promiseClasses(state: string): string {
        switch (state) {
            case "fulfilled":
                return "bg-emerald-100 text-emerald-700";
            case "rejected":
                return "bg-red-100 text-red-700";
            default:
                return "bg-amber-100 text-amber-700";
        }
    }
</script>

<div
    class="rounded-md border px-2 py-1.5 transition-all {synthetic
        ? 'border-amber-300 bg-amber-50'
        : 'border-slate-200 bg-white'} {top ? 'border-4' : ''}"
>
    <div class="flex min-w-0 flex-wrap items-center gap-1">
        <span
            class="shrink-0 rounded border px-1 py-0.5 text-[9px] font-semibold uppercase leading-none {typeClasses(
                value.type,
                synthetic,
            )}"
        >
            {synthetic ? "JIT" : value.type}
        </span>
        {#if value.promise}
            <span
                class="shrink-0 rounded px-1 py-0.5 text-[9px] font-semibold leading-none {promiseClasses(
                    value.promise.state,
                )}"
            >
                {value.promise.state}
            </span>
        {/if}
        {#if value.function?.async}
            <span
                class="shrink-0 rounded bg-indigo-100 px-1 py-0.5 text-[9px] font-semibold leading-none text-indigo-600"
            >
                async
            </span>
        {/if}
        {#if value.id}
            <span class="ml-auto shrink-0 text-[9px] text-slate-400"
                >{value.id}</span
            >
        {/if}
    </div>
    <p
        class="mt-1 break-words font-mono text-xs leading-snug {synthetic
            ? 'text-amber-800'
            : 'text-slate-700'} {compact ? 'line-clamp-2' : ''}"
        title={value.display}
    >
        {value.display}
    </p>
    {#if value.promise?.value}
        <p class="mt-1 truncate font-mono text-[10px] text-slate-500">
            → {value.promise.value.display}
        </p>
    {/if}
</div>
