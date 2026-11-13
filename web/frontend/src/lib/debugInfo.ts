import type {
  DebugInfo,
  DisassembledChunk,
  InterpretDetails,
  LegacyDisassembledChunk,
} from "../types";

function normalizeChunk(
  chunk: Partial<DisassembledChunk>,
  index: number,
): DisassembledChunk {
  return {
    id: chunk.id ?? `chunk-${index}`,
    name: chunk.name ?? `chunk-${index}`,
    kind: chunk.kind,
    hidden: chunk.hidden,
    byte_code: Array.isArray(chunk.byte_code) ? chunk.byte_code : [],
    jit: Array.isArray(chunk.jit) ? chunk.jit : [],
  };
}

function fromLegacy(
  debugInfo: Record<string, LegacyDisassembledChunk>,
): DisassembledChunk[] {
  return Object.entries(debugInfo).map(([name, chunk], index) => ({
    id: `legacy:${name}:${index}`,
    name,
    hidden: name === "INTERNAL_SETUP",
    byte_code: Array.isArray(chunk?.byte_code) ? chunk.byte_code : [],
    jit: Array.isArray(chunk?.jit) ? chunk.jit : [],
  }));
}

export function normalizeDebugChunks(
  details: Partial<InterpretDetails> | null | undefined,
): DisassembledChunk[] {
  const direct = details?.debug_chunks;
  if (Array.isArray(direct)) {
    return direct.map(normalizeChunk);
  }

  const debugInfo = details?.debug_info as
    | DebugInfo
    | Record<string, LegacyDisassembledChunk>
    | undefined;

  const nested = (debugInfo as DebugInfo | undefined)?.chunks;
  if (Array.isArray(nested)) {
    return nested.map(normalizeChunk);
  }

  if (debugInfo !== null && typeof debugInfo === "object") {
    return fromLegacy(debugInfo as Record<string, LegacyDisassembledChunk>);
  }

  return [];
}
