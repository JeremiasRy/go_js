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

export function generateLookUp(node: AstNode): Record<number, AstNode> {
    const lookUp: Record<number, AstNode> = {}
    const recurse = (node: AstNode) => {
        lookUp[node.id] = node
        for (const obj of Object.values(node)) {
            if (Array.isArray(obj)) {
                for (const node of obj) {
                    recurse(node)
                }
                continue;
            }
            if (objectIsAstNode(obj)) {
                recurse(obj as AstNode)
            }
        }

    }
    recurse(node)
    return lookUp
}