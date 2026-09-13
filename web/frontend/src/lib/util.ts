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

export function generateLookUp(node: AstNode): Map<number, AstNode> {
    const lookUp: Map<number, AstNode> = new Map();
    const recurse = (node: unknown) => {
        if (!objectIsAstNode(node)) {
            return
        }

        const astNode = node as AstNode
        if (!lookUp.has(astNode.id)) {
            lookUp.set(astNode.id, astNode)
        }
        for (const obj of Object.values(astNode)) {
            Array.isArray(obj)
                ? obj.forEach(recurse)
                : recurse(obj)
        }
    }
    recurse(node)
    console.log(lookUp)
    return lookUp
}