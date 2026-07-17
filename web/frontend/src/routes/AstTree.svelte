<script module lang="ts">
    const _expansionState: { [key: number]: Boolean } = $state({});
</script>

<script lang="ts">
    import { slide } from "svelte/transition";
    import type { AstNode, HighlightType } from "../types";
    import AstTree from "./AstTree.svelte";
    import Arrow from "./Arrow.svelte";

    let {
        node,
        setHighlight,
        highlight,
    }: {
        node: AstNode;
        setHighlight: (highlight: HighlightType) => void;
        highlight: HighlightType;
    } = $props();
    let expanded = $derived(_expansionState[node.id] || false);

    const camelToCapital = (str: string) => {
        return str
            .replace(/([A-Z])/g, " $1")
            .replace(/^./, (str) => str.toUpperCase());
    };
    const splitCapitalCase = (str: string) => {
        return str.replace(/(?<!^)(?=[A-Z])/g, " ");
    };

    const toggleExpansion = () => {
        expanded = _expansionState[node.id] = !expanded;
    };

    const propIsAstNode = (prop: unknown): boolean => {
        if (typeof prop === "object" && prop !== null) {
            return (
                "id" in prop &&
                "type" in prop &&
                "start" in prop &&
                "end" in prop
            );
        }

        return false;
    };

    const skipProp = ([prop]: [string, unknown]): boolean => {
        return !["type", "start", "end", "id", "ast_train"].includes(prop);
    };

    const onMouseEnter = (e: Event) => {
        e.stopPropagation();
        setHighlight({
            ast_ids: [...(node.ast_train ?? []), node.id],
            from: node.start,
            to: node.end,
        });
    };

    const onMouseLeave = (e: Event) => {
        e.stopPropagation();
        setHighlight({ ast_ids: [], from: 0, to: 0 });
    };

    $effect(() => {
        if (
            highlight.ast_ids.length === 1 &&
            highlight.from === 0 &&
            highlight.to === 0 &&
            node.start !== 0 &&
            node.end !== 0
        ) {
            setHighlight({
                ast_ids: highlight.ast_ids,
                from: node.start,
                to: node.end,
            });
        }
    });
</script>

<ul
    class="ast-node border-solid border-l p-1 mb-1 rounded-md {String(node.id)}"
    onmouseover={onMouseEnter}
    onmouseleave={onMouseLeave}
    onfocus={() => {}}
    onblur={() => {}}
>
    <li>
        <button onclick={toggleExpansion}>
            <Arrow direction={expanded ? "down" : "right"} />
            <span class="font-semibold">{splitCapitalCase(node.type)}</span>
        </button>

        {#if expanded}
            <ul transition:slide class="ml-4">
                {#each Object.entries(node).filter(skipProp) as [key, value]}
                    {#if Array.isArray(value)}
                        <i class="my-2">{camelToCapital(key)}</i>
                        {#each value as possibleNode}
                            {#if propIsAstNode(possibleNode as object)}
                                <AstTree
                                    node={possibleNode as AstNode}
                                    {highlight}
                                    {setHighlight}
                                />
                            {/if}
                        {/each}
                    {:else if propIsAstNode(value as object)}
                        <i>{camelToCapital(key)}</i>
                        <AstTree
                            node={value as AstNode}
                            {highlight}
                            {setHighlight}
                        />
                    {:else if typeof value === "object" && value !== null}
                        <div>
                            {#each Object.entries(value) as [k, v]}
                                <p>{camelToCapital(k)}: <i>{v}</i></p>
                            {/each}
                        </div>
                    {:else}
                        <div>
                            <span>{camelToCapital(key)}: <b>{value}</b></span>
                        </div>
                    {/if}
                {/each}
            </ul>
        {/if}
    </li>
</ul>

<style>
    .ast-node {
        border-color: #e5e7eb;
        background-color: transparent;
    }

    .ast-node:hover:not(:has(.ast-node:hover)) {
        border-color: #3b82f6;
        background-color: #eff6ff;
    }
</style>
