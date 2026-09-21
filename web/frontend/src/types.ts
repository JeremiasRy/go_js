export interface AstNode {
  [key: string]: unknown;
  id: number;
  start: number;
  end: number;
  type: string;
  ast_train: number[];
}

export type HighlightStatus = {
  source: "ast" | "op_code";
  astId: number;
  astIds: number[];
};
export type PageStatus = "input" | "submitting" | "polling" | "error" | "done";
export type JobStatus = "Success" | "Failed" | "Pending" | "Processing";
export type FunctionName = string;
export type DisassembledByteCode =
  { byte_code: { ast_id: number; op: string }[]; jit: { inst: string; op_code_ptr: number; ast_id: number } }


export type InterpretDetails = {
  output: string;
  debug_info: Record<FunctionName, DisassembledByteCode>;
  ast: AstNode;
};
export type InterpretResult = {
  jobStatus: JobStatus;
  result: InterpretDetails | null;
};

