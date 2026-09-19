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
