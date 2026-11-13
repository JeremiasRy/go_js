export interface AstNode {
  [key: string]: unknown;
  id: number;
  start: number;
  end: number;
  type: string;
  ast_train: number[];
}

export type HighlightStatus = {
  source: "ast" | "op_code" | "asm";
  astId: number;
  astIds: number[];
  opCodePtr: number;
  chunkId: string;
}

type setHighlightFromOpCode = {
  source: "op_code";
  astId: number;
  opCodePtr: number;
  chunkId: string;
}

type setHighlightFromAsm = {
  source: "asm";
  astId: number;
  opCodePtr: number;
  chunkId: string;
}

type setHighlightFromAst = {
  source: "ast";
  astId: number;
}

export type SetHighlightStatusParams = setHighlightFromOpCode | setHighlightFromAsm | setHighlightFromAst | null

export type PageStatus = "input" | "submitting" | "running" | "error" | "done";
export type JobStatus = "Success" | "Failed" | "Pending" | "Processing";

export type ScriptJob = {
  job_id: string;
  session_id: string;
  src: string;
  job_status: JobStatus;
  created_at: string;
};

export type DisassembledOp = {
  ast_id: number;
  op: string;
  op_ptr: number;
};

export type DisassembledJit = {
  inst: string;
  op_code_ptr: number;
  ast_id: number;
};

export type DisassembledChunk = {
  id: string;
  name: string;
  kind?: string;
  hidden?: boolean;
  byte_code: DisassembledOp[];
  jit: DisassembledJit[];
};

export type ExecutionLimits = {
  max_frames: number;
  max_stack: number;
  max_queue_items: number;
  max_timers: number;
  max_value_chars: number;
};

export type ExecutionInfo = {
  schema_version: number;
  status: "not_requested" | "complete" | "truncated" | "error" | string;
  frames: unknown[];
  truncated: boolean;
  total_frames: number;
  dropped_frames: number;
  duration_ms: number;
  limits: ExecutionLimits;
};

export type InterpretDetails = {
  format_version: number;
  output: string;
  debug_chunks: DisassembledChunk[];
  ast: AstNode;
  execution?: ExecutionInfo;
};

export type InterpretResult = {
  jobStatus: JobStatus;
  result: InterpretDetails | null;
};
