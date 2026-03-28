import { useMemo } from "react";
import { Badge, Tabs, Tooltip, Typography } from "antd";
import type { ParsedSheet } from "../../api/template";
import type { DataViewDef } from "../../lib/configTypes";
import { colIndexToLetter } from "../../lib/cellRef";
import {
  extractPlaceholders,
  type PlaceholderLocation,
} from "../../lib/templatePlaceholders";

const { Text } = Typography;

const PLACEHOLDER_RE = /\{(\w+)\}/g;

interface SheetGridProps {
  sheet: ParsedSheet;
  matchedLabels: Set<string>;
}

function SheetGrid({ sheet, matchedLabels }: SheetGridProps) {
  const { rows, maxCol } = sheet;

  const placeholderCells = useMemo(() => {
    const cells = new Set<string>();
    rows.forEach((row, ri) => {
      row.forEach((cell, ci) => {
        if (PLACEHOLDER_RE.test(cell)) cells.add(`${ri}-${ci}`);
        PLACEHOLDER_RE.lastIndex = 0;
      });
    });
    return cells;
  }, [rows]);

  return (
    <div className="overflow-auto max-h-64 border border-gray-200 rounded text-xs">
      <table className="border-collapse min-w-full">
        <thead>
          <tr>
            <th className="w-8 min-w-8 bg-gray-100 border border-gray-200 px-1 py-0.5 text-center text-gray-400 font-normal sticky left-0 z-10" />
            {Array.from({ length: maxCol }, (_, i) => (
              <th
                key={i}
                className="min-w-20 bg-gray-100 border border-gray-200 px-2 py-0.5 text-center text-gray-500 font-normal"
              >
                {colIndexToLetter(i)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <tr key={ri}>
              <td className="bg-gray-100 border border-gray-200 px-1 py-0.5 text-center text-gray-400 sticky left-0 z-10">
                {ri + 1}
              </td>
              {Array.from({ length: maxCol }, (_, ci) => {
                const val = row[ci] ?? "";
                const isPlaceholder = placeholderCells.has(`${ri}-${ci}`);
                const placeholderNames = isPlaceholder
                  ? [...val.matchAll(/\{(\w+)\}/g)].map((m) => m[1])
                  : [];
                const allMatched =
                  placeholderNames.length > 0 &&
                  placeholderNames.every((n) => matchedLabels.has(n));
                const someUnmatched = placeholderNames.some(
                  (n) => !matchedLabels.has(n),
                );

                const bg = isPlaceholder
                  ? allMatched
                    ? "bg-green-50"
                    : someUnmatched
                      ? "bg-orange-50"
                      : "bg-blue-50"
                  : "";

                return (
                  <td
                    key={ci}
                    className={`border border-gray-200 px-2 py-0.5 whitespace-nowrap font-mono ${bg}`}
                  >
                    {isPlaceholder ? (
                      <Tooltip
                        title={
                          allMatched
                            ? "Matched to a label"
                            : "No matching label in Step 2"
                        }
                      >
                        <span
                          className={
                            allMatched ? "text-green-700" : "text-orange-600"
                          }
                        >
                          {val}
                        </span>
                      </Tooltip>
                    ) : (
                      val
                    )}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

interface TemplatePreviewProps {
  sheets: ParsedSheet[];
  dataViews: DataViewDef[];
  showLabelMappings?: boolean;
}

export default function TemplatePreview({
  sheets,
  dataViews,
  showLabelMappings = false,
}: TemplatePreviewProps) {
  const allLabelNames = useMemo(() => {
    const s = new Set<string>();
    dataViews.forEach((dv) => dv.labels.forEach((l) => l.name && s.add(l.name)));
    return s;
  }, [dataViews]);

  const allPlaceholders = useMemo(() => {
    const map = new Map<string, PlaceholderLocation[]>();
    sheets.forEach((sheet) => {
      extractPlaceholders(sheet.rows).forEach((locs, name) => {
        if (!map.has(name)) map.set(name, []);
        map.get(name)!.push(...locs);
      });
    });
    return map;
  }, [sheets]);

  const matched = useMemo(
    () => [...allPlaceholders.keys()].filter((n) => allLabelNames.has(n)).sort(),
    [allPlaceholders, allLabelNames],
  );
  const unmatched = useMemo(
    () => [...allPlaceholders.keys()].filter((n) => !allLabelNames.has(n)).sort(),
    [allPlaceholders, allLabelNames],
  );

  const mappedViews = useMemo(
    () =>
      dataViews
        .filter((dv) => dv.name || dv.dataSource || dv.labels.some((l) => l.name || l.column))
        .map((dv) => ({
          ...dv,
          labels: dv.labels.filter((label) => label.name || label.column),
        })),
    [dataViews],
  );

  const tabItems = sheets.map((sheet) => ({
    key: sheet.name,
    label: sheet.name,
    children: <SheetGrid sheet={sheet} matchedLabels={allLabelNames} />,
  }));

  return (
    <div className="flex flex-col gap-3">
      <Tabs size="small" items={tabItems} />

      {allPlaceholders.size > 0 && (
        <div className="flex flex-wrap gap-2 text-xs">
          {matched.map((name) => (
            <Badge
              key={name}
              color="green"
              text={<Text className="text-xs">{`{${name}}`}</Text>}
            />
          ))}
          {unmatched.map((name) => (
            <Tooltip key={name} title="No matching label defined in Step 2">
              <Badge
                color="orange"
                text={<Text className="text-xs text-orange-600">{`{${name}}`}</Text>}
              />
            </Tooltip>
          ))}
        </div>
      )}

      {showLabelMappings && mappedViews.length > 0 && (
        <div className="flex flex-col gap-3">
          <Text strong className="text-sm">Label Mappings</Text>
          <div className="grid gap-3 md:grid-cols-2">
            {mappedViews.map((dv) => (
              <div
                key={`${dv.name}-${dv.dataSource}`}
                className="border border-gray-200 rounded-md p-3 bg-white"
              >
                <div className="flex items-center justify-between gap-3 mb-2">
                  <Text strong>{dv.name || "Unnamed Data View"}</Text>
                  <Text type="secondary" className="text-xs">
                    {dv.dataSource || "No data source"}
                  </Text>
                </div>
                <table className="w-full border-collapse text-xs">
                  <thead>
                    <tr>
                      <th className="text-left text-gray-500 font-normal border-b border-gray-200 pb-1 pr-2">
                        Label
                      </th>
                      <th className="text-left text-gray-500 font-normal border-b border-gray-200 pb-1">
                        Column
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {dv.labels.length > 0 ? (
                      dv.labels.map((label, index) => (
                        <tr key={`${label.name}-${label.column}-${index}`}>
                          <td className="py-1 pr-2 font-mono align-top">{label.name || "-"}</td>
                          <td className="py-1 font-mono align-top">{label.column || "-"}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={2} className="py-1 text-gray-400">
                          No labels configured
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
