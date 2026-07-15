export interface AstNode {
  [key: string]: unknown;
  id: number;
  start: number;
  end: number;
  type: string;
  ast_train: number[];
}

export type HighlightType = {
  from: number;
  to: number;
  ast_ids: number[];
};
