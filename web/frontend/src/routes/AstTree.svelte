<script module lang="ts">
    const _expansionState: { [key: number]: Boolean } = $state({});
</script>

<script lang="ts">
    type PropsType = {
        node: AstNode;
        setHighlight: (
            opts: {
                astId: number;
                source: "ast" | "op_code";
            } | null,
        ) => void;
        highlight: HighlightStatus | null;
    };
    import { slide } from "svelte/transition";
    import type { AstNode, HighlightStatus } from "../types";
    import AstTree from "./AstTree.svelte";
    import Arrow from "./Arrow.svelte";
    import { objectIsAstNode } from "$lib/util";

    let { node, setHighlight, highlight }: PropsType = $props();
    let expanded = $derived(_expansionState[node.id] || false);

    $effect(() => {
        if (highlight !== null && highlight.source === "op_code") {
            expanded = true;
        }
    });

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

    const skipProp = ([prop]: [string, unknown]): boolean => {
        return !["type", "start", "end", "id", "ast_train"].includes(prop);
    };

    const onMouseEnter = (e: Event) => {
        e.stopPropagation();
        setHighlight({ source: "ast", astId: node.id });
    };

    const onMouseLeave = (e: Event) => {
        e.stopPropagation();
        setHighlight(null);
    };

    const determineHighlightStatus = () => {
        if (
            highlight !== null &&
            highlight.source === "op_code" &&
            highlight.astId === node.id
        ) {
            return "border-color: #3b82f6; background-color: #eff6ff;";
        }
        return "";
    };
</script>

<ul
    class="ast-node border-solid border-l p-1 mb-1 rounded-md {String(node.id)}"
    onmouseover={onMouseEnter}
    onmouseleave={onMouseLeave}
    onfocus={() => {}}
    onblur={() => {}}
    style={determineHighlightStatus()}
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
                            {#if objectIsAstNode(possibleNode)}
                                <AstTree
                                    node={possibleNode as AstNode}
                                    {highlight}
                                    {setHighlight}
                                />
                            {/if}
                        {/each}
                    {:else if objectIsAstNode(value)}
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
