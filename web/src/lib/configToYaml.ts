import yaml from "js-yaml";
import type { BundleConfig, BlockDef } from "./configTypes";

function cleanBlock(block: BlockDef): Record<string, unknown> {
  const out: Record<string, unknown> = {
    name: block.name,
    type: block.type,
    range: { ref: block.range.ref },
  };

  if (block.dataView) out.dataView = block.dataView;
  if (block.direction) out.direction = block.direction;
  if (block.rowLimit && block.rowLimit > 0) out.rowLimit = block.rowLimit;
  if (block.insertAfter) out.insertAfter = true;
  if (block.labelVariable) out.labelVariable = block.labelVariable;
  if (block.template) out.template = true;

  if (block.subBlocks && block.subBlocks.length > 0) {
    out.subBlocks = block.subBlocks.map(cleanBlock);
  }

  return out;
}

export function configToYaml(config: BundleConfig): string {
  const bundle: Record<string, unknown> = {
    workbook: {
      id: config.workbook.id,
      name: config.workbook.name,
      template: config.workbook.template,
      outputDir: config.workbook.outputDir,
      ...(config.workbook.parameters &&
        Object.keys(config.workbook.parameters).length > 0 && {
          parameters: config.workbook.parameters,
        }),
      sheets: config.workbook.sheets.map((sheet) => {
        const s: Record<string, unknown> = { name: sheet.name };
        if (sheet.dynamic) {
          s.dynamic = true;
          if (sheet.dataView) s.dataView = sheet.dataView;
          if (sheet.paramLabel) s.paramLabel = sheet.paramLabel;
        }
        if (sheet.verticalArrangement) s.verticalArrangement = true;
        if (sheet.allowOverlap) s.allowOverlap = true;
        if (sheet.blocks.length > 0) {
          s.blocks = sheet.blocks.map(cleanBlock);
        }
        return s;
      }),
    },
    dataViews: config.dataViews.map((dv) => ({
      name: dv.name,
      dataSource: dv.dataSource,
      labels: dv.labels.map((l) => ({ name: l.name, column: l.column })),
    })),
    dataSources: config.dataSources.map((ds) => ({
      name: ds.name,
      driver: ds.driver,
      dsn: ds.dsn,
    })),
  };

  return yaml.dump(bundle, {
    indent: 2,
    lineWidth: 120,
    noRefs: true,
    quotingType: '"',
    forceQuotes: false,
  });
}
