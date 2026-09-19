import type { AstNode } from "../types";

export function objectIsAstNode(prop: unknown): boolean {
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

export function generateLookUp({ node, target }: { node: AstNode, target: Map<number, AstNode> }) {
    const recurse = (node: unknown) => {
        if (!objectIsAstNode(node)) {
            return
        }

        const astNode = node as AstNode
        if (!target.has(astNode.id)) {
            target.set(astNode.id, astNode)
        }
        for (const obj of Object.values(astNode)) {
            Array.isArray(obj)
                ? obj.forEach(recurse)
                : recurse(obj)
        }
    }
    recurse(node)
}