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

export type HighlightOptions = {
  scroll?: boolean;
  toggle?: boolean;
};

export type ExecutionLimits = {
  max_frames: number;
  max_stack: number;
  max_queue_items: number;
  max_timers: number;
  max_value_chars: number;
};

export type OutputDelta = {
  delta: string;
  length: number;
};

export type ValueView = {
  type: string;
  display: string;
  id?: string;
  synthetic?: boolean;
  promise?: PromiseView;
  function?: FunctionView;
};

export type PromiseView = {
  state: "pending" | "fulfilled" | "rejected";
  value?: ValueView;
  reactions?: number;
  observers?: number;
};

export type FunctionView = {
  name: string;
  arity: number;
  async?: boolean;
};

export type VMStackFrame = {
  index: number;
  fn: string;
  chunk_id: string;
  op_ptr: number;
  op: string;
  ast_id: number;
  local_start: number;
};

export type JITState = {
  mode: "call" | "osr";
  state: "running" | "done" | "deopt" | "throw";
  fn: string;
  op_ptr: number;
  result?: ValueView;
  duration_ms?: number;
};

export type VMState = {
  id: number;
  parent_id?: number;
  active_frame: number;
  op_ptr: number;
  op: string;
  ast_id: number;
  fn: string;
  chunk_id: string;
  frames: VMStackFrame[];
  frames_omitted?: number;
  stack: ValueView[];
  stack_base: number;
  stack_top: number;
  stack_omitted?: number;
  jit?: JITState;
};

export type QueueItemKind = "task" | "micro_task";
export type QueueName = "tasks" | "micro_tasks";

export type QueueItem = {
  id: string;
  kind: QueueItemKind;
  label: string;
  source?: string;
  fn?: string;
  native?: boolean;
};

export type QueueState = {
  micro_tasks: QueueItem[];
  tasks: QueueItem[];
  micro_tasks_omitted?: number;
  tasks_omitted?: number;
};

export type TimerItem = {
  id: number;
  delay_ms: number;
  label: string;
  repeats?: boolean;
  state: "pending" | "fired" | "cancelled";
};

export type EventLoopState = {
  phase:
    | "booting"
    | "executing"
    | "queued"
    | "waiting"
    | "idle"
    | "done";
  next_queue?: QueueName;
  current_job?: QueueItem;
  work: number;
  timers: TimerItem[];
  timers_omitted?: number;
};

export type ExecutionEventType =
  | "enqueue"
  | "job_start"
  | "job_end"
  | "waiting"
  | "timer_scheduled"
  | "timer_cancelled"
  | "await_suspend"
  | "jit_enter"
  | "jit_exit"
  | "done"
  | "error";

export type ExecutionEvent = {
  type: ExecutionEventType;
  queue?: QueueName;
  item?: QueueItem;
  fn?: string;
  resume_op_ptr?: number;
  promise?: ValueView;
  timer?: TimerItem;
  detail?: string;
};

export type ExecutionFrameKind = "boot" | "step" | "event" | "jit" | "end";

export type ExecutionFrame = {
  seq: number;
  kind: ExecutionFrameKind;
  at_ms: number;
  note?: string;
  event?: ExecutionEvent;
  vm?: VMState;
  event_loop: EventLoopState;
  queues: QueueState;
  output?: OutputDelta;
};

export type ExecutionInfo = {
  schema_version: number;
  status: "not_requested" | "complete" | "truncated" | "error" | string;
  frames: ExecutionFrame[];
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
