/** "A" → 0, "B" → 1, "AA" → 26 */
export function colLetterToIndex(col: string): number {
  let n = 0;
  for (const c of col.toUpperCase()) {
    n = n * 26 + (c.charCodeAt(0) - 64);
  }
  return n - 1;
}

/** 0 → "A", 25 → "Z", 26 → "AA" */
export function colIndexToLetter(index: number): string {
  let result = "";
  let n = index + 1;
  while (n > 0) {
    const rem = (n - 1) % 26;
    result = String.fromCharCode(65 + rem) + result;
    n = Math.floor((n - 1) / 26);
  }
  return result;
}

export interface CellCoord {
  row: number; // 0-based
  col: number; // 0-based
}

/** "A1" → { row: 0, col: 0 }, "C5" → { row: 4, col: 2 } */
export function parseCellRef(ref: string): CellCoord | null {
  const m = ref.trim().match(/^([A-Za-z]+)(\d+)$/);
  if (!m) return null;
  return { col: colLetterToIndex(m[1]), row: parseInt(m[2], 10) - 1 };
}

export interface RangeCoords {
  startRow: number;
  startCol: number;
  endRow: number;
  endCol: number;
}

/** "A1:C5" → { startRow: 0, startCol: 0, endRow: 4, endCol: 2 } */
export function parseRangeRef(ref: string): RangeCoords | null {
  const parts = ref.trim().split(":");
  if (parts.length !== 2) return null;
  const start = parseCellRef(parts[0]);
  const end = parseCellRef(parts[1]);
  if (!start || !end) return null;
  return {
    startRow: start.row,
    startCol: start.col,
    endRow: end.row,
    endCol: end.col,
  };
}

/** Extract a sub-grid from rows for the given range string. Returns null if ref is invalid. */
export function getCellsInRange(rows: string[][], rangeRef: string): string[][] | null {
  const coords = parseRangeRef(rangeRef);
  if (!coords) return null;
  const { startRow, startCol, endRow, endCol } = coords;
  const result: string[][] = [];
  for (let r = startRow; r <= endRow; r++) {
    const row: string[] = [];
    for (let c = startCol; c <= endCol; c++) {
      row.push(rows[r]?.[c] ?? "");
    }
    result.push(row);
  }
  return result;
}
