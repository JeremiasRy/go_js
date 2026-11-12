// Shared module fixture: named/default exports and module-local closures.
export const number = 7;
const label = "fixture";
export { label as title };
export let mutable = 1;
const local = "exports-local";
export function increment() { return ++mutable; }
export function readLocal() { return local; }
export function add(a, b) { return a + b; }
export default function describe(value) { return "default:" + value; }
