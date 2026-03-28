import type { ParsedSheet } from "../api/template";
import type { DataViewDef } from "./configTypes";

const PLACEHOLDER_RE = /\{(\w+)\}/g;

export interface PlaceholderLocation {
  row: number;
  col: number;
}

export interface TemplatePlaceholderStatus {
  matched: string[];
  unmatched: string[];
}

export function extractPlaceholders(
  rows: string[][],
): Map<string, PlaceholderLocation[]> {
  const map = new Map<string, PlaceholderLocation[]>();
  rows.forEach((row, ri) => {
    row.forEach((cell, ci) => {
      for (const match of cell.matchAll(PLACEHOLDER_RE)) {
        const name = match[1];
        if (!map.has(name)) map.set(name, []);
        map.get(name)!.push({ row: ri, col: ci });
      }
    });
  });
  return map;
}

export function getTemplatePlaceholderStatus(
  sheets: ParsedSheet[],
  dataViews: DataViewDef[],
): TemplatePlaceholderStatus {
  const allLabelNames = new Set<string>();
  dataViews.forEach((dv) =>
    dv.labels.forEach((label) => {
      if (label.name) {
        allLabelNames.add(label.name);
      }
    }),
  );

  const allPlaceholders = new Set<string>();
  sheets.forEach((sheet) => {
    extractPlaceholders(sheet.rows).forEach((_, name) => {
      allPlaceholders.add(name);
    });
  });

  const matched: string[] = [];
  const unmatched: string[] = [];

  [...allPlaceholders.keys()].sort().forEach((name) => {
    if (allLabelNames.has(name)) {
      matched.push(name);
    } else {
      unmatched.push(name);
    }
  });

  return { matched, unmatched };
}