import { Button, Card, Form, Input, Select, Space, Typography } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { snakeCase } from "lodash";
import type { DataSourceDef, DataViewDef, LabelDef } from "../../lib/configTypes";

const { Text } = Typography;

interface DataViewStepProps {
  dataViews: DataViewDef[];
  dataSources: DataSourceDef[];
  csvHeaders: Record<string, string[]>;
  onChange: (dataViews: DataViewDef[]) => void;
}

export default function DataViewStep({
  dataViews,
  dataSources,
  csvHeaders,
  onChange,
}: DataViewStepProps) {
  const dsOptions = dataSources
    .filter((ds) => ds.name)
    .map((ds) => ({ label: ds.name, value: ds.name }));

  const updateView = (index: number, field: keyof DataViewDef, value: unknown) => {
    const next = [...dataViews];
    next[index] = { ...next[index], [field]: value };
    onChange(next);
  };

  const handleDataSourceChange = (index: number, dataSource: string) => {
    const next = [...dataViews];
    const current = next[index];
    const shouldSyncName = !current.name || current.name === current.dataSource;

    next[index] = {
      ...current,
      dataSource,
      name: shouldSyncName ? dataSource : current.name,
    };

    onChange(next);
  };

  const updateLabel = (
    viewIdx: number,
    labelIdx: number,
    field: keyof LabelDef,
    value: string,
  ) => {
    const next = [...dataViews];
    const labels = [...next[viewIdx].labels];
    labels[labelIdx] = { ...labels[labelIdx], [field]: value };
    next[viewIdx] = { ...next[viewIdx], labels };
    onChange(next);
  };

  const addView = () => {
    onChange([
      ...dataViews,
      { name: "", dataSource: "", labels: [{ name: "", column: "" }] },
    ]);
  };

  const removeView = (index: number) => {
    onChange(dataViews.filter((_, i) => i !== index));
  };

  const addLabel = (viewIdx: number) => {
    const next = [...dataViews];
    next[viewIdx] = {
      ...next[viewIdx],
      labels: [...next[viewIdx].labels, { name: "", column: "" }],
    };
    onChange(next);
  };

  const removeLabel = (viewIdx: number, labelIdx: number) => {
    const next = [...dataViews];
    next[viewIdx] = {
      ...next[viewIdx],
      labels: next[viewIdx].labels.filter((_, i) => i !== labelIdx),
    };
    onChange(next);
  };

  const mapAllColumns = (viewIdx: number, headers: string[]) => {
    const next = [...dataViews];
    next[viewIdx] = {
      ...next[viewIdx],
      labels: headers.map((h) => ({ name: snakeCase(h), column: h })),
    };
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-4">
      {dataViews.map((dv, vIdx) => {
        const headers = csvHeaders[dv.dataSource] ?? [];
        const columnOptions = headers.map((h) => ({ value: h }));

        return (
        <Card
          key={vIdx}
          size="small"
          title={`Data View ${vIdx + 1}`}
          extra={
            <Button
              type="text"
              danger
              icon={<DeleteOutlined />}
              onClick={() => removeView(vIdx)}
            />
          }
        >
          <Form layout="vertical" size="small">
            <Form.Item
              label="Name"
              required
              validateStatus={dv.name ? "" : "error"}
            >
              <Input
                placeholder="e.g. employee_view"
                value={dv.name}
                onChange={(e) => updateView(vIdx, "name", e.target.value)}
              />
            </Form.Item>
            <Form.Item
              label="Data Source"
              required
              validateStatus={dv.dataSource ? "" : "error"}
            >
              <Select
                placeholder="Select a data source"
                options={dsOptions}
                value={dv.dataSource || undefined}
                onChange={(v) => handleDataSourceChange(vIdx, v)}
              />
            </Form.Item>

            <div className="mb-2 flex items-center gap-2">
              <Text strong>Labels (column mappings)</Text>
              {headers.length > 0 && (
                <Button
                  type="link"
                  size="small"
                  onClick={() => mapAllColumns(vIdx, headers)}
                >
                  Map All Columns
                </Button>
              )}
            </div>
            {dv.labels.map((label, lIdx) => (
              <Space key={lIdx} className="flex mb-2" align="start">
                <Input
                  placeholder="Label name"
                  value={label.name}
                  onChange={(e) =>
                    updateLabel(vIdx, lIdx, "name", e.target.value)
                  }
                />
                <Select
                  placeholder="Column name"
                  value={label.column || undefined}
                  options={columnOptions}
                  onChange={(v) => updateLabel(vIdx, lIdx, "column", v)}
                  showSearch
                  filterOption={(input, option) =>
                    (option?.value ?? "")
                      .toLowerCase()
                      .includes(input.toLowerCase())
                  }
                  style={{ width: 200 }}
                />
                {dv.labels.length > 1 && (
                  <Button
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => removeLabel(vIdx, lIdx)}
                  />
                )}
              </Space>
            ))}
            <Button
              type="dashed"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => addLabel(vIdx)}
            >
              Add Label
            </Button>
          </Form>
        </Card>
        );
      })}
      <Button type="dashed" icon={<PlusOutlined />} onClick={addView} block>
        Add Data View
      </Button>
    </div>
  );
}
