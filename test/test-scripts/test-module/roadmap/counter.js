// Shared module fixture: mutable live bindings and isolated lexical state.
export let count = 0;
const local = "counter-local";
export function increment() { return ++count; }
export function readLocal() { return local; }
