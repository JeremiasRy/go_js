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
        registerThySelf: (opts: { id: number; el: HTMLElement }) => void;
    };
    import { slide } from "svelte/transition";
    import type { AstNode, HighlightStatus } from "../types";
    import AstTree from "./AstTree.svelte";
    import Arrow from "./Arrow.svelte";
    import { objectIsAstNode } from "$lib/util";

    const { node, setHighlight, highlight, registerThySelf }: PropsType =
        $props();

    const camelToCapital = (str: string) => {
        return str
            .replace(/([A-Z])/g, " $1")
            .replace(/^./, (str) => str.toUpperCase());
    };
    const splitCapitalCase = (str: string) => {
        return str.replace(/(?<!^)(?=[A-Z])/g, " ");
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
    bind:this={
        () => {},
        (el) => {
            registerThySelf({ id: Number(node.id), el });
        }
    }
>
    <li>
        <Arrow direction={"down"} />
        <span class="font-semibold">{splitCapitalCase(node.type)}</span>

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
                                {registerThySelf}
                            />
                        {/if}
                    {/each}
                {:else if objectIsAstNode(value)}
                    <i>{camelToCapital(key)}</i>
                    <AstTree
                        node={value as AstNode}
                        {highlight}
                        {setHighlight}
                        {registerThySelf}
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
    </li>
</ul>

<style>
    .ast-node {
        border-color: #e5e7eb;
        background-color: transparent;
        scroll-margin-top: 0.5em;
    }

    .ast-node:hover:not(:has(.ast-node:hover)) {
        border-color: #3b82f6;
        background-color: #eff6ff;
    }
</style>
