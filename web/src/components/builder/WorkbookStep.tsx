import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Collapse,
  Form,
  Input,
  InputNumber,
  Select,
  Spin,
  Switch,
  Typography,
  Upload,
} from "antd";
import { PlusOutlined, DeleteOutlined, FileExcelOutlined } from "@ant-design/icons";
import { camelCase } from "lodash";
import type { UploadFile } from "antd";
import type {
  WorkbookDef,
  SheetDef,
  BlockDef,
  DataViewDef,
} from "../../lib/configTypes";
import { parseTemplate } from "../../api/template";
import type { ParseTemplateResponse, ParsedSheet } from "../../api/template";
import TemplatePreview from "./TemplatePreview";
import { getCellsInRange, colIndexToLetter } from "../../lib/cellRef";
import type { TemplatePlaceholderStatus } from "../../lib/templatePlaceholders";

const { Text } = Typography;
const { Dragger } = Upload;

interface WorkbookStepProps {
  workbook: WorkbookDef;
  dataViews: DataViewDef[];
  templateFile: File | null;
  parsedTemplate: ParseTemplateResponse | null;
  placeholderStatus: TemplatePlaceholderStatus;
  headerLabelVariableIssues: string[];
  onChange: (workbook: WorkbookDef) => void;
  onTemplateChange: (file: File | null) => void;
  onParsedTemplateChange: (data: ParseTemplateResponse | null) => void;
}

// Header blocks are only valid as matrix sub-blocks; top-level blocks use value/matrix only.
const topLevelBlockTypeOptions = [
  { label: "Value", value: "value" },
  { label: "Matrix", value: "matrix" },
];

// Inside a matrix's data sub-blocks, value and nested matrix are allowed.
const dataSubBlockTypeOptions = [
  { label: "Value", value: "value" },
  { label: "Matrix (nested)", value: "matrix" },
];

const directionOptions = [
  { label: "Vertical", value: "vertical" },
  { label: "Horizontal", value: "horizontal" },
];

const defaultMatrixSubBlocks = (): BlockDef[] => [
  {
    name: "RowHeader",
    type: "header",
    direction: "vertical",
    insertAfter: true,
    range: { ref: "" },
  },
  {
    name: "ColHeader",
    type: "header",
    direction: "horizontal",
    range: { ref: "" },
  },
  {
    name: "Data",
    type: "value",
    template: true,
    range: { ref: "" },
  },
];

function RangePreview({
  rangeRef,
  sheet,
}: {
  rangeRef: string;
  sheet: ParsedSheet | undefined;
}) {
  if (!sheet || !rangeRef.includes(":")) return null;
  const cells = getCellsInRange(sheet.rows, rangeRef);
  if (!cells || cells.length === 0) return null;

  return (
    <div className="mt-1 overflow-auto max-h-24 border border-gray-200 rounded text-xs">
      <table className="border-collapse">
        <tbody>
          {cells.map((row, ri) => (
            <tr key={ri}>
              {row.map((cell, ci) => (
                <td
                  key={ci}
                  className="border border-gray-200 px-2 py-0.5 font-mono whitespace-nowrap"
                >
                  {cell || <span className="text-gray-300">empty</span>}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Inline form for a header sub-block (direction is fixed, shown as read-only label). */
function HeaderSubForm({
  label,
  block,
  dataViews,
  sheetData,
  onChange,
}: {
  label: string;
  block: BlockDef;
  dataViews: DataViewDef[];
  sheetData: ParsedSheet | undefined;
  onChange: (b: BlockDef) => void;
}) {
  const dvOptions = dataViews
    .filter((dv) => dv.name)
    .map((dv) => ({ label: dv.name, value: dv.name }));
  const hasRange = block.range.ref.trim() !== "";
  const selectedDataView = dataViews.find((dv) => dv.name === block.dataView);
  const labelOptions = (block.dataView
    ? selectedDataView?.labels.map((label) => ({
        label: `${label.name} (${label.column})`,
        value: label.name,
        dataViewName: selectedDataView.name,
        labelName: label.name,
      })) ?? []
    : dataViews.flatMap((dv) =>
        dv.labels.map((label) => ({
          label: `${dv.name} / ${label.name} (${label.column})`,
          value: `${dv.name}::${label.name}`,
          dataViewName: dv.name,
          labelName: label.name,
        })),
      ))
    .filter((option) => option.labelName && option.dataViewName);

  const update = (field: string, value: unknown) =>
    onChange({ ...block, [field]: value });

  const handleDataViewChange = (nextDataView: string | undefined) => {
    const dataViewName = nextDataView ?? "";
    const availableLabels = new Set(
      (dataViews.find((dv) => dv.name === dataViewName)?.labels ?? [])
        .map((label) => label.name)
        .filter((name): name is string => Boolean(name)),
    );

    onChange({
      ...block,
      dataView: dataViewName,
      labelVariable: availableLabels.has(block.labelVariable ?? "")
        ? block.labelVariable
        : "",
    });
  };

  const handleLabelVariableChange = (value: string) => {
    const selectedOption = labelOptions.find((option) => option.value === value);
    if (!selectedOption) {
      return;
    }

    onChange({
      ...block,
      dataView: selectedOption.dataViewName,
      labelVariable: selectedOption.labelName,
    });
  };

  return (
    <Card size="small" title={label} className="border-dashed">
      <Form layout="vertical" size="small">
        <div className="grid grid-cols-2 gap-x-4">
          <Form.Item label="Name">
            <Input
              placeholder="e.g. RowHeader"
              value={block.name}
              onChange={(e) => update("name", e.target.value)}
            />
          </Form.Item>
          <Form.Item label="Range (ref)" required>
            <Input
              placeholder="e.g. A3:A3"
              value={block.range.ref}
              onChange={(e) => update("range", { ref: e.target.value })}
            />
            <RangePreview rangeRef={block.range.ref} sheet={sheetData} />
          </Form.Item>
          <Form.Item label="Data View">
            <Select
              allowClear
              placeholder="(optional)"
              options={dvOptions}
              value={block.dataView || undefined}
              onChange={(v) =>
                handleDataViewChange(typeof v === "string" ? v : undefined)
              }
            />
          </Form.Item>
          {hasRange && (
            <Form.Item
              label="Label Variable"
              required
              validateStatus={block.labelVariable ? "" : "error"}
              help={
                block.labelVariable
                  ? undefined
                  : labelOptions.length > 0
                    ? "Choose one of the label mappings from Data Setup."
                    : "No label mappings available yet. Configure labels in Data Setup first."
              }
            >
              <Select
                showSearch
                allowClear
                placeholder={
                  block.dataView
                    ? "Select a label variable"
                    : "Choose from existing label mappings"
                }
                options={labelOptions}
                value={block.labelVariable || undefined}
                onChange={(v) => {
                  if (typeof v === "string") {
                    handleLabelVariableChange(v);
                  }
                }}
                filterOption={(input, option) =>
                  `${option?.label ?? ""} ${option?.value ?? ""}`
                    .toLowerCase()
                    .includes(input.toLowerCase())
                }
              />
            </Form.Item>
          )}
        </div>
        <Form.Item label="Insert After">
          <Switch
            checked={block.insertAfter ?? false}
            onChange={(v) => update("insertAfter", v)}
          />
        </Form.Item>
      </Form>
    </Card>
  );
}

function BlockForm({
  block,
  dataViews,
  sheetData,
  onChange,
  onRemove,
  depth = 0,
}: {
  block: BlockDef;
  dataViews: DataViewDef[];
  sheetData: ParsedSheet | undefined;
  onChange: (block: BlockDef) => void;
  onRemove: () => void;
  depth?: number;
}) {
  const dvOptions = dataViews
    .filter((dv) => dv.name)
    .map((dv) => ({ label: dv.name, value: dv.name }));

  const typeOptions = depth > 0 ? dataSubBlockTypeOptions : topLevelBlockTypeOptions;

  const updateBlock = (field: string, value: unknown) =>
    onChange({ ...block, [field]: value });

  const handleTypeChange = (newType: string) => {
    const base = { ...block, type: newType as BlockDef["type"] };
    if (newType === "matrix" && (!block.subBlocks || block.subBlocks.length === 0)) {
      base.subBlocks = defaultMatrixSubBlocks();
    }
    onChange(base);
  };

  // For matrix: split sub-blocks into fixed headers and user-managed data blocks
  const subBlocks = block.subBlocks ?? [];
  const rowHeader = subBlocks.find(
    (s) => s.type === "header" && s.direction === "vertical",
  ) ?? defaultMatrixSubBlocks()[0];
  const colHeader = subBlocks.find(
    (s) => s.type === "header" && s.direction === "horizontal",
  ) ?? defaultMatrixSubBlocks()[1];
  const dataBlocks = subBlocks.filter((s) => s.type !== "header");

  const updateHeader = (direction: "vertical" | "horizontal", updated: BlockDef) => {
    const next = subBlocks.filter((s) => !(s.type === "header" && s.direction === direction));
    onChange({ ...block, subBlocks: [...next, updated] });
  };

  const updateDataBlock = (idx: number, updated: BlockDef) => {
    const nextData = [...dataBlocks];
    nextData[idx] = updated;
    const headers = subBlocks.filter((s) => s.type === "header");
    onChange({ ...block, subBlocks: [...headers, ...nextData] });
  };

  const addDataBlock = () => {
    const headers = subBlocks.filter((s) => s.type === "header");
    onChange({
      ...block,
      subBlocks: [
        ...headers,
        ...dataBlocks,
        { name: "", type: "value", range: { ref: "" } },
      ],
    });
  };

  const removeDataBlock = (idx: number) => {
    const headers = subBlocks.filter((s) => s.type === "header");
    onChange({
      ...block,
      subBlocks: [...headers, ...dataBlocks.filter((_, i) => i !== idx)],
    });
  };

  return (
    <Card
      size="small"
      className={depth > 0 ? "border-dashed" : ""}
      title={block.name || "Unnamed Block"}
      extra={
        <Button type="text" danger icon={<DeleteOutlined />} onClick={onRemove} />
      }
    >
      <Form layout="vertical" size="small">
        <div className="grid grid-cols-2 gap-x-4">
          <Form.Item label="Name" required>
            <Input
              placeholder="e.g. DataBlock1"
              value={block.name}
              onChange={(e) => updateBlock("name", e.target.value)}
            />
          </Form.Item>
          <Form.Item label="Type" required>
            <Select
              options={typeOptions}
              value={block.type}
              onChange={handleTypeChange}
            />
          </Form.Item>
          <Form.Item label="Range (ref)" required>
            <Input
              placeholder="e.g. A1:B5"
              value={block.range.ref}
              onChange={(e) => updateBlock("range", { ref: e.target.value })}
            />
            <RangePreview rangeRef={block.range.ref} sheet={sheetData} />
          </Form.Item>
          <Form.Item label="Data View">
            <Select
              allowClear
              placeholder="(optional)"
              options={dvOptions}
              value={block.dataView || undefined}
              onChange={(v) => updateBlock("dataView", v ?? "")}
            />
          </Form.Item>
        </div>

        {block.type === "value" && (
          <>
            <Form.Item label="Direction">
              <Select
                options={directionOptions}
                value={block.direction || undefined}
                onChange={(v) => updateBlock("direction", v)}
                placeholder="(optional)"
                allowClear
              />
            </Form.Item>
            <div className="grid grid-cols-3 gap-x-4">
              <Form.Item label="Row Limit">
                <InputNumber
                  min={0}
                  placeholder="0 = no limit"
                  value={block.rowLimit ?? undefined}
                  onChange={(v) => updateBlock("rowLimit", v ?? 0)}
                  className="w-full"
                />
              </Form.Item>
              <Form.Item label="Insert After">
                <Switch
                  checked={block.insertAfter ?? false}
                  onChange={(v) => updateBlock("insertAfter", v)}
                />
              </Form.Item>
              <Form.Item label="Template">
                <Switch
                  checked={block.template ?? false}
                  onChange={(v) => updateBlock("template", v)}
                />
              </Form.Item>
            </div>
          </>
        )}

        {block.type === "matrix" && (
          <div className="flex flex-col gap-3 mt-2">
            <Text strong>Row Header (vertical axis)</Text>
            <HeaderSubForm
              label="Row Header"
              block={rowHeader}
              dataViews={dataViews}
              sheetData={sheetData}
              onChange={(updated) => updateHeader("vertical", { ...updated, type: "header", direction: "vertical" })}
            />

            <Text strong>Column Header (horizontal axis)</Text>
            <HeaderSubForm
              label="Column Header"
              block={colHeader}
              dataViews={dataViews}
              sheetData={sheetData}
              onChange={(updated) => updateHeader("horizontal", { ...updated, type: "header", direction: "horizontal" })}
            />

            <Text strong>Data Blocks</Text>
            <div className="flex flex-col gap-2">
              {dataBlocks.map((sub, idx) => (
                <BlockForm
                  key={idx}
                  block={sub}
                  dataViews={dataViews}
                  sheetData={sheetData}
                  onChange={(updated) => updateDataBlock(idx, updated)}
                  onRemove={() => removeDataBlock(idx)}
                  depth={depth + 1}
                />
              ))}
              <Button
                type="dashed"
                size="small"
                icon={<PlusOutlined />}
                onClick={addDataBlock}
              >
                Add Data Block
              </Button>
            </div>
          </div>
        )}
      </Form>
    </Card>
  );
}

export default function WorkbookStep({
  workbook,
  dataViews,
  templateFile,
  parsedTemplate,
  placeholderStatus,
  headerLabelVariableIssues,
  onChange,
  onTemplateChange,
  onParsedTemplateChange,
}: WorkbookStepProps) {
  const [parsing, setParsing] = useState(false);
  const [parseError, setParseError] = useState<string | null>(null);

  const dvOptions = dataViews
    .filter((dv) => dv.name)
    .map((dv) => ({ label: dv.name, value: dv.name }));

  const updateField = (field: string, value: unknown) => {
    onChange({ ...workbook, [field]: value });
  };

  const handleTemplateUpload = async (file: File) => {
    onTemplateChange(file);
    updateField("template", file.name);
    setParseError(null);
    setParsing(true);
    try {
      const result = await parseTemplate(file);
      onParsedTemplateChange(result);
      // Auto-populate sheets from template if still at default
      const isDefault =
        workbook.sheets.length === 1 &&
        (workbook.sheets[0].name === "" || workbook.sheets[0].name === "Sheet1") &&
        workbook.sheets[0].blocks.length === 0;
      if (isDefault && result.sheets.length > 0) {
        onChange({
          ...workbook,
          template: file.name,
          sheets: result.sheets.map((s) => ({ name: s.name, blocks: [] })),
        });
      }
    } catch (err) {
      setParseError(err instanceof Error ? err.message : "Failed to parse template");
      onParsedTemplateChange(null);
    } finally {
      setParsing(false);
    }
  };

  const updateSheet = (index: number, sheet: SheetDef) => {
    const sheets = [...workbook.sheets];
    sheets[index] = sheet;
    onChange({ ...workbook, sheets });
  };

  const addSheet = () => {
    onChange({
      ...workbook,
      sheets: [...workbook.sheets, { name: "", blocks: [] }],
    });
  };

  const removeSheet = (index: number) => {
    onChange({
      ...workbook,
      sheets: workbook.sheets.filter((_, i) => i !== index),
    });
  };

  const updateBlockInSheet = (
    sheetIdx: number,
    blockIdx: number,
    block: BlockDef,
  ) => {
    const sheets = [...workbook.sheets];
    const blocks = [...sheets[sheetIdx].blocks];
    blocks[blockIdx] = block;
    sheets[sheetIdx] = { ...sheets[sheetIdx], blocks };
    onChange({ ...workbook, sheets });
  };

  const addBlockToSheet = (sheetIdx: number) => {
    const sheets = [...workbook.sheets];
    sheets[sheetIdx] = {
      ...sheets[sheetIdx],
      blocks: [
        ...sheets[sheetIdx].blocks,
        { name: "", type: "value", range: { ref: "" } },
      ],
    };
    onChange({ ...workbook, sheets });
  };

  const removeBlockFromSheet = (sheetIdx: number, blockIdx: number) => {
    const sheets = [...workbook.sheets];
    sheets[sheetIdx] = {
      ...sheets[sheetIdx],
      blocks: sheets[sheetIdx].blocks.filter((_, i) => i !== blockIdx),
    };
    onChange({ ...workbook, sheets });
  };

  return (
    <div className="flex flex-col gap-4">
      <Card size="small" title="Workbook Settings">
        <Form layout="vertical" size="small">
          <Form.Item label="Workbook Name" required>
            <Input
              placeholder="e.g. Monthly Report"
              value={workbook.name}
              onChange={(e) => {
                const name = e.target.value;
                onChange({ ...workbook, name, id: camelCase(name) });
              }}
            />
          </Form.Item>
          <Form.Item label="Template File (.xlsx)" required>
            <Dragger
              accept=".xlsx"
              maxCount={1}
              fileList={
                templateFile
                  ? [
                      {
                        uid: "tpl",
                        name: templateFile.name,
                        status: "done",
                      } as UploadFile,
                    ]
                  : []
              }
              beforeUpload={(file) => {
                handleTemplateUpload(file);
                return false;
              }}
              onRemove={() => {
                onTemplateChange(null);
                onParsedTemplateChange(null);
                updateField("template", "");
              }}
            >
              <p className="text-2xl text-gray-400">
                <FileExcelOutlined />
              </p>
              <Text className="text-xs">
                Drop your Excel template here or click to browse
              </Text>
            </Dragger>
          </Form.Item>
        </Form>

        {parsing && (
          <div className="flex items-center gap-2 text-sm text-gray-500 mt-2">
            <Spin size="small" /> Parsing template...
          </div>
        )}
        {parseError && (
          <Alert
            type="error"
            message={parseError}
            showIcon
            closable
            className="mt-2"
            onClose={() => setParseError(null)}
          />
        )}
        {parsedTemplate && (
          <div className="mt-4">
            <Text strong className="text-sm">Template Preview</Text>
            <div className="mt-2">
              <TemplatePreview
                sheets={parsedTemplate.sheets}
                dataViews={dataViews}
                showLabelMappings
              />
            </div>
          </div>
        )}
        {parsedTemplate && placeholderStatus.unmatched.length > 0 && (
          <Alert
            type="warning"
            message="Unmatched Template Placeholders"
            description={`Add mappings for these labels in Data Setup before continuing: ${placeholderStatus.unmatched.join(", ")}`}
            showIcon
            className="mt-4"
          />
        )}
        {headerLabelVariableIssues.length > 0 && (
          <Alert
            type="warning"
            message="Header Label Variable Required"
            description={
              <div className="text-sm">
                <div>Choose a Label Variable for each configured header block:</div>
                <div className="mt-1">
                  {headerLabelVariableIssues.join("; ")}
                </div>
              </div>
            }
            showIcon
            className="mt-4"
          />
        )}
      </Card>

      <Text strong className="text-base">
        Sheets
      </Text>

      <Collapse
        accordion
        items={workbook.sheets.map((sheet, sIdx) => {
          const sheetData = parsedTemplate?.sheets.find(
            (s) => s.name === sheet.name,
          );
          return {
            key: String(sIdx),
            label: sheet.name || `Sheet ${sIdx + 1}`,
            extra: (
              <Button
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                onClick={(e) => {
                  e.stopPropagation();
                  removeSheet(sIdx);
                }}
              />
            ),
            children: (
              <div className="flex flex-col gap-3">
                {sheetData && (
                  <div>
                    <Text type="secondary" className="text-xs">
                      Template cells ({sheetData.maxRow} rows ×{" "}
                      {colIndexToLetter(sheetData.maxCol - 1)} cols)
                    </Text>
                    <div className="mt-1">
                      <TemplatePreview
                        sheets={[sheetData]}
                        dataViews={dataViews}
                      />
                    </div>
                  </div>
                )}
                <Form layout="vertical" size="small">
                  <Form.Item label="Sheet Name" required>
                    <Input
                      placeholder="e.g. Sheet1"
                      value={sheet.name}
                      onChange={(e) =>
                        updateSheet(sIdx, { ...sheet, name: e.target.value })
                      }
                    />
                  </Form.Item>
                  <div className="flex gap-4">
                    <Form.Item label="Dynamic">
                      <Switch
                        checked={sheet.dynamic ?? false}
                        onChange={(v) =>
                          updateSheet(sIdx, { ...sheet, dynamic: v })
                        }
                      />
                    </Form.Item>
                    {sheet.dynamic && (
                      <>
                        <Form.Item label="Data View" required className="flex-1">
                          <Select
                            options={dvOptions}
                            value={sheet.dataView || undefined}
                            onChange={(v) =>
                              updateSheet(sIdx, { ...sheet, dataView: v })
                            }
                          />
                        </Form.Item>
                        <Form.Item
                          label="Param Label"
                          required
                          className="flex-1"
                        >
                          <Input
                            placeholder="e.g. month_label"
                            value={sheet.paramLabel ?? ""}
                            onChange={(e) =>
                              updateSheet(sIdx, {
                                ...sheet,
                                paramLabel: e.target.value,
                              })
                            }
                          />
                        </Form.Item>
                      </>
                    )}
                  </div>
                </Form>

                <Text strong>Blocks</Text>
                <div className="flex flex-col gap-2">
                  {sheet.blocks.map((block, bIdx) => (
                    <BlockForm
                      key={bIdx}
                      block={block}
                      dataViews={dataViews}
                      sheetData={sheetData}
                      onChange={(updated) =>
                        updateBlockInSheet(sIdx, bIdx, updated)
                      }
                      onRemove={() => removeBlockFromSheet(sIdx, bIdx)}
                    />
                  ))}
                  <Button
                    type="dashed"
                    icon={<PlusOutlined />}
                    onClick={() => addBlockToSheet(sIdx)}
                  >
                    Add Block
                  </Button>
                </div>
              </div>
            ),
          };
        })}
      />

      <Button type="dashed" icon={<PlusOutlined />} onClick={addSheet} block>
        Add Sheet
      </Button>
    </div>
  );
}
