import { createContext } from "svelte";

interface HighlightContext {
  from: number;
  to: number;
}

export const [getHighlightContext, setHighlightContext] =
  createContext<HighlightContext>();
