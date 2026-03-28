import { useMemo, useState } from "react";
import { Button, Steps } from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router";
import { useBeforeunload } from "react-beforeunload";
import DataSourceStep from "../components/builder/DataSourceStep";
import type { CsvMeta } from "../components/builder/DataSourceStep";
import DataViewStep from "../components/builder/DataViewStep";
import WorkbookStep from "../components/builder/WorkbookStep";
import YamlPreviewStep from "../components/builder/YamlPreviewStep";
import type { PreparedDataFile } from "../components/builder/YamlPreviewStep";
import { configToYaml } from "../lib/configToYaml";
import type { ParseTemplateResponse } from "../api/template";
import type {
  BlockDef,
  DataSourceDef,
  DataViewDef,
  WorkbookDef,
} from "../lib/configTypes";
import { getTemplatePlaceholderStatus } from "../lib/templatePlaceholders";

function collectHeaderLabelVariableIssues(
  blocks: BlockDef[],
  path: string[],
): string[] {
  const issues: string[] = [];

  blocks.forEach((block) => {
    const blockName = block.name || "Unnamed Block";
    const nextPath = [...path, blockName];

    if (
      block.type === "header" &&
      block.range.ref.trim() !== "" &&
      !block.labelVariable?.trim()
    ) {
      issues.push(nextPath.join(" > "));
    }

    if (block.subBlocks?.length) {
      issues.push(...collectHeaderLabelVariableIssues(block.subBlocks, nextPath));
    }
  });

  return issues;
}

const defaultWorkbook: WorkbookDef = {
  id: "",
  name: "",
  template: "",
  outputDir: "output",
  sheets: [{ name: "Sheet1", blocks: [] }],
};

export default function ConfigBuilderPage() {
  const navigate = useNavigate();
  const [current, setCurrent] = useState(0);
  const [dataSources, setDataSources] = useState<DataSourceDef[]>([
    { name: "", driver: "csv", dsn: "local" },
  ]);
  const [csvMeta, setCsvMeta] = useState<CsvMeta[]>([
    { file: null, headers: [] },
  ]);
  const [dataViews, setDataViews] = useState<DataViewDef[]>([
    { name: "", dataSource: "", labels: [{ name: "", column: "" }] },
  ]);
  const [workbook, setWorkbook] = useState<WorkbookDef>(defaultWorkbook);
  const [templateFile, setTemplateFile] = useState<File | null>(null);
  const [parsedTemplate, setParsedTemplate] =
    useState<ParseTemplateResponse | null>(null);

  const csvHeaders = useMemo(() => {
    const map: Record<string, string[]> = {};
    dataSources.forEach((ds, i) => {
      if (ds.name && csvMeta[i]?.headers.length) {
        map[ds.name] = csvMeta[i].headers;
      }
    });
    return map;
  }, [dataSources, csvMeta]);

  const preparedDataFiles = useMemo<PreparedDataFile[]>(() => {
    const dataSourceFiles = new Map<string, File>();

    dataSources.forEach((dataSource, index) => {
      const file = csvMeta[index]?.file;
      if (dataSource.name && file) {
        dataSourceFiles.set(dataSource.name, file);
      }
    });

    return dataViews
      .filter((dataView) => dataView.name && dataView.dataSource)
      .map((dataView) => {
        const sourceFile = dataSourceFiles.get(dataView.dataSource) ?? null;
        const targetName = `${dataView.name}.csv`;

        if (!sourceFile) {
          return {
            viewName: dataView.name,
            targetName,
            sourceName: null,
            file: null,
          };
        }

        const file =
          sourceFile.name === targetName
            ? sourceFile
            : new File([sourceFile], targetName, {
                type: sourceFile.type || "text/csv",
                lastModified: sourceFile.lastModified,
              });

        return {
          viewName: dataView.name,
          targetName,
          sourceName: sourceFile.name,
          file,
        };
      });
  }, [dataSources, csvMeta, dataViews]);

  const yamlContent = useMemo(
    () =>
      configToYaml({
        workbook,
        dataViews: dataViews.filter((dv) => dv.name),
        dataSources: dataSources.filter((ds) => ds.name),
      }),
    [workbook, dataViews, dataSources],
  );

  const placeholderStatus = useMemo(
    () =>
      parsedTemplate
        ? getTemplatePlaceholderStatus(parsedTemplate.sheets, dataViews)
        : { matched: [], unmatched: [] },
    [parsedTemplate, dataViews],
  );

  const headerLabelVariableIssues = useMemo(
    () =>
      workbook.sheets.flatMap((sheet) =>
        collectHeaderLabelVariableIssues(sheet.blocks, [sheet.name || "Unnamed Sheet"]),
      ),
    [workbook],
  );

  const canProceedFromDS = dataSources.some(
    (ds) => ds.name && ds.driver && ds.dsn,
  );
  const canProceedFromDV = dataViews.some(
    (dv) => dv.name && dv.dataSource && dv.labels.some((l) => l.name && l.column),
  );
  const canProceedFromDataSetup = canProceedFromDS && canProceedFromDV;
  const canProceedFromWB =
    workbook.id &&
    workbook.name &&
    workbook.template &&
    templateFile &&
    parsedTemplate !== null &&
    placeholderStatus.unmatched.length === 0 &&
    headerLabelVariableIssues.length === 0 &&
    workbook.sheets.length > 0;

  const steps = [
    { title: "Data Setup", description: "Upload CSVs & map columns" },
    { title: "Workbook", description: "Sheets & blocks" },
    { title: "Preview & Generate", description: "Review YAML" },
  ];

  const canProceed = [canProceedFromDataSetup, canProceedFromWB, true];

  const isDirty = useMemo(() => {
    const hasDataSourceChanges = dataSources.some(
      (ds) => ds.name.trim() !== "" || ds.driver !== "csv" || ds.dsn !== "local",
    );
    const hasCsvFile = csvMeta.some((meta) => meta.file !== null);
    const hasDataViewChanges = dataViews.some(
      (dv) =>
        dv.name.trim() !== "" ||
        dv.dataSource.trim() !== "" ||
        dv.labels.some((label) => label.name.trim() !== "" || label.column.trim() !== ""),
    );
    const hasWorkbookChanges =
      workbook.id.trim() !== "" ||
      workbook.name.trim() !== "" ||
      workbook.template.trim() !== "" ||
      workbook.outputDir !== defaultWorkbook.outputDir ||
      workbook.sheets.length !== defaultWorkbook.sheets.length ||
      workbook.sheets.some((sheet, index) => {
        const defaultSheet = defaultWorkbook.sheets[index];
        if (!defaultSheet) {
          return true;
        }
        return sheet.name !== defaultSheet.name || sheet.blocks.length > 0;
      });

    return (
      hasDataSourceChanges ||
      hasCsvFile ||
      hasDataViewChanges ||
      hasWorkbookChanges ||
      templateFile !== null ||
      parsedTemplate !== null
    );
  }, [dataSources, csvMeta, dataViews, workbook, templateFile, parsedTemplate]);

  useBeforeunload(
    isDirty ? (event: BeforeUnloadEvent) => event.preventDefault() : undefined,
  );

  const handleBackToHome = () => {
    if (!isDirty) {
      navigate("/");
      return;
    }

    window.location.assign("/");
  };

  return (
    <div className="max-w-3xl mx-auto p-8">
      <div className="flex items-center gap-3 mb-6">
        <Button
          type="text"
          icon={<ArrowLeftOutlined />}
          onClick={handleBackToHome}
        />
        <h1 className="text-2xl font-semibold m-0">Config Builder</h1>
      </div>

      <Steps current={current} items={steps} className="mb-8" />

      <div className="min-h-100">
        {current === 0 && (
          <div className="flex flex-col gap-8">
            <section className="flex flex-col gap-4">
              <div>
                <h2 className="text-lg font-medium m-0">Data Sources</h2>
                <p className="text-sm text-gray-500 mt-1 mb-0">
                  Upload the CSV files first so the next section can use their detected columns.
                </p>
              </div>
              <DataSourceStep
                dataSources={dataSources}
                csvMeta={csvMeta}
                onChange={(ds, meta) => {
                  setDataSources(ds);
                  setCsvMeta(meta);
                }}
              />
            </section>

            <section className="flex flex-col gap-4">
              <div>
                <h2 className="text-lg font-medium m-0">Data Views</h2>
                <p className="text-sm text-gray-500 mt-1 mb-0">
                  Define the labels here so Workbook configuration can reference them directly in the next step.
                </p>
              </div>
              <DataViewStep
                dataViews={dataViews}
                dataSources={dataSources}
                csvHeaders={csvHeaders}
                onChange={setDataViews}
              />
            </section>
          </div>
        )}
        {current === 1 && (
          <WorkbookStep
            workbook={workbook}
            dataViews={dataViews}
            templateFile={templateFile}
            parsedTemplate={parsedTemplate}
            placeholderStatus={placeholderStatus}
            headerLabelVariableIssues={headerLabelVariableIssues}
            onChange={setWorkbook}
            onTemplateChange={setTemplateFile}
            onParsedTemplateChange={setParsedTemplate}
          />
        )}
        {current === 2 && (
          <YamlPreviewStep
            yamlContent={yamlContent}
            templateFile={templateFile}
            preparedDataFiles={preparedDataFiles}
          />
        )}
      </div>

      {current < steps.length && (
        <div className="flex justify-between mt-8">
          <Button
            disabled={current === 0}
            onClick={() => setCurrent((c) => c - 1)}
          >
            Back
          </Button>
          {current < steps.length - 1 && (
            <Button
              type="primary"
              disabled={!canProceed[current]}
              onClick={() => setCurrent((c) => c + 1)}
            >
              Next
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
