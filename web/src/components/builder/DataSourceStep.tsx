import { Button, Card, Form, Input, Typography, Upload } from "antd";
import {
  PlusOutlined,
  DeleteOutlined,
  FileTextOutlined,
} from "@ant-design/icons";
import type { UploadFile } from "antd";
import type { DataSourceDef } from "../../lib/configTypes";

export interface CsvMeta {
  file: File | null;
  headers: string[];
}

const { Text } = Typography;
const { Dragger } = Upload;

interface DataSourceStepProps {
  dataSources: DataSourceDef[];
  csvMeta: CsvMeta[];
  onChange: (dataSources: DataSourceDef[], csvMeta: CsvMeta[]) => void;
}

function parseCsvHeaders(text: string): string[] {
  const firstLine = text.split(/\r?\n/)[0];
  if (!firstLine) return [];
  return firstLine.split(",").map((h) => h.trim().replace(/^"|"$/g, ""));
}

export default function DataSourceStep({
  dataSources,
  csvMeta,
  onChange,
}: DataSourceStepProps) {
  const updateItem = (
    index: number,
    field: keyof DataSourceDef,
    value: string,
  ) => {
    const next = [...dataSources];
    next[index] = { ...next[index], [field]: value };
    onChange(next, csvMeta);
  };

  const updateCsv = (index: number, file: File | null, headers: string[]) => {
    const nextMeta = [...csvMeta];
    nextMeta[index] = { file, headers };
    onChange(dataSources, nextMeta);
  };

  const handleFileUpload = (index: number, file: File) => {
    const reader = new FileReader();
    reader.onload = () => {
      const headers = parseCsvHeaders(reader.result as string);
      const nextMeta = [...csvMeta];
      nextMeta[index] = { file, headers };

      // Auto-fill name from filename if empty
      const nextDS = [...dataSources];
      if (!nextDS[index].name) {
        const name = file.name.replace(/\.csv$/i, "");
        nextDS[index] = { ...nextDS[index], name };
      }

      onChange(nextDS, nextMeta);
    };
    // Only read first 4KB to get headers
    reader.readAsText(file.slice(0, 4096));
  };

  const addItem = () => {
    onChange(
      [...dataSources, { name: "", driver: "csv", dsn: "local" }],
      [...csvMeta, { file: null, headers: [] }],
    );
  };

  const removeItem = (index: number) => {
    onChange(
      dataSources.filter((_, i) => i !== index),
      csvMeta.filter((_, i) => i !== index),
    );
  };

  return (
    <div className="flex flex-col gap-4">
      {dataSources.map((ds, index) => {
        const meta = csvMeta[index] ?? { file: null, headers: [] };
        return (
          <Card
            key={index}
            size="small"
            title={`Data Source ${index + 1}`}
            extra={
              dataSources.length > 1 && (
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => removeItem(index)}
                />
              )
            }
          >
            <Form layout="vertical" size="small">
              <Form.Item label="CSV File">
                <Dragger
                  accept=".csv"
                  maxCount={1}
                  fileList={
                    meta.file
                      ? [
                          {
                            uid: `csv-${index}`,
                            name: meta.file.name,
                            status: "done",
                          } as UploadFile,
                        ]
                      : []
                  }
                  beforeUpload={(file) => {
                    handleFileUpload(index, file);
                    return false;
                  }}
                  onRemove={() => updateCsv(index, null, [])}
                >
                  <p className="text-2xl text-gray-400">
                    <FileTextOutlined />
                  </p>
                  <Text className="text-xs">
                    Drop a CSV file here or click to browse
                  </Text>
                </Dragger>
              </Form.Item>

              {meta.headers.length > 0 && (
                <div className="mb-3 text-xs text-gray-500">
                  <Text type="secondary">
                    Columns detected: {meta.headers.join(", ")}
                  </Text>
                </div>
              )}

              <Form.Item
                label="Name"
                required
                validateStatus={ds.name ? "" : "error"}
                help={ds.name ? undefined : "Name is required"}
              >
                <Input
                  placeholder="e.g. local_csv (auto-filled from filename)"
                  value={ds.name}
                  onChange={(e) => updateItem(index, "name", e.target.value)}
                />
              </Form.Item>
            </Form>
          </Card>
        );
      })}
      <Button type="dashed" icon={<PlusOutlined />} onClick={addItem} block>
        Add Data Source
      </Button>
    </div>
  );
}
