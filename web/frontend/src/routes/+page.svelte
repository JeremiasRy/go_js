<script lang="ts">
    import { basicSetup } from "codemirror";
    import { EditorView } from "@codemirror/view";
    import { javascript } from "@codemirror/lang-javascript";
    import { onMount } from "svelte";

    let editorContainer: HTMLElement;
    let view: EditorView;

    onMount(() => {
        view = new EditorView({
            doc: `
// JS Editor -- Write your code snippet here and send it off to my interpreter for evaluation :)
// P.S. please don't hack me...
function fib(n) {
    if (n <= 1) {
        return n;
    }
    return fib(n - 1) + fib(n - 2);
}

const startTime = Date.now();
const result = fib(40);
const endTime = Date.now();

console.log(\`Result: \${result}\`);
console.log(\`Execution Time: \${endTime - startTime}ms\`);`,
            parent: editorContainer,
            extensions: [basicSetup, javascript()],
        });
    });

    async function submitCode() {
        const src = view.state.doc.toString();
        const result = await fetch("/api/interpret", {
            method: "POST",
            body: JSON.stringify({ src }),
        });

        const json = await result.json();
        console.log({ json });
    }
</script>

<div class="w-full">
    <div bind:this={editorContainer}></div>
    <button onclick={submitCode}>Submit</button>
</div>
