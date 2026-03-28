export interface DataSourceDef {
  name: string;
  driver: string;
  dsn: string;
}

export interface LabelDef {
  name: string;
  column: string;
}

export interface DataViewDef {
  name: string;
  dataSource: string;
  labels: LabelDef[];
}

export interface CellRangeDef {
  ref: string;
}

export type BlockType = "value" | "header" | "matrix";
export type Direction = "vertical" | "horizontal";

export interface BlockDef {
  name: string;
  type: BlockType;
  range: CellRangeDef;
  dataView?: string;
  direction?: Direction;
  rowLimit?: number;
  insertAfter?: boolean;
  labelVariable?: string;
  template?: boolean;
  subBlocks?: BlockDef[];
}

export interface SheetDef {
  name: string;
  dynamic?: boolean;
  dataView?: string;
  paramLabel?: string;
  verticalArrangement?: boolean;
  allowOverlap?: boolean;
  blocks: BlockDef[];
}

export interface WorkbookDef {
  id: string;
  name: string;
  template: string;
  outputDir: string;
  parameters?: Record<string, string>;
  sheets: SheetDef[];
}

export interface BundleConfig {
  workbook: WorkbookDef;
  dataViews: DataViewDef[];
  dataSources: DataSourceDef[];
}
