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
  fn: string;
}

type setHighlightFromOpCode = {
  source: "op_code";
  astId: number;
  opCodePtr: number;
  fn: string;
}

type setHighlightFromAsm = {
  source: "asm";
  astId: number;
  opCodePtr: number;
  fn: string;
}

type setHighlightFromAst = {
  source: "ast";
  astId: number;
}

export type SetHighlightStatusParams = setHighlightFromOpCode | setHighlightFromAsm | setHighlightFromAst | null

export type PageStatus = "input" | "submitting" | "polling" | "error" | "done";
export type JobStatus = "Success" | "Failed" | "Pending" | "Processing";
export type FunctionName = string;
export type DisassembledByteCode =
  { byte_code: { ast_id: number; op: string; op_ptr: number }[]; jit: { inst: string; op_code_ptr: number; ast_id: number }[] }


export type InterpretDetails = {
  output: string;
  debug_info: Record<FunctionName, DisassembledByteCode>;
  ast: AstNode;
};
export type InterpretResult = {
  jobStatus: JobStatus;
  result: InterpretDetails | null;
};

