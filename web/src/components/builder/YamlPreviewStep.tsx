import { useState } from "react";
import { Button, Typography, message, Alert, Space } from "antd";
import {
  CopyOutlined,
  DownloadOutlined,
  SendOutlined,
} from "@ant-design/icons";
import { generateReport, downloadBlob } from "../../api/generate";

const { Title, Text } = Typography;

export interface PreparedDataFile {
  viewName: string;
  targetName: string;
  sourceName: string | null;
  file: File | null;
}

interface YamlPreviewStepProps {
  yamlContent: string;
  templateFile: File | null;
  preparedDataFiles?: PreparedDataFile[];
}

export default function YamlPreviewStep({
  yamlContent,
  templateFile,
  preparedDataFiles = [],
}: YamlPreviewStepProps) {
  const dataFiles = preparedDataFiles
    .map((item) => item.file)
    .filter((file): file is File => file !== null);
  const missingFiles = preparedDataFiles.filter((item) => item.file === null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(yamlContent);
    message.success("YAML copied to clipboard");
  };

  const handleDownloadYaml = () => {
    const blob = new Blob([yamlContent], { type: "text/yaml" });
    downloadBlob(blob, "config.yaml");
  };

  const handleGenerate = async () => {
    if (!templateFile || dataFiles.length === 0 || missingFiles.length > 0) {
      message.error("Please upload both a template and data files");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const configFile = new File([yamlContent], "config.yaml", {
        type: "text/yaml",
      });

      const blob = await generateReport({
        config: configFile,
        template: templateFile,
        dataFiles,
      });

      const name = templateFile.name.replace(/\.xlsx$/, "");
      downloadBlob(blob, `${name}_output.xlsx`);
      message.success("Report generated and downloaded!");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "An unknown error occurred",
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <div>
        <div className="flex justify-between items-center mb-2">
          <Title level={5} className="mb-0!">
            Generated YAML
          </Title>
          <Space>
            <Button size="small" icon={<CopyOutlined />} onClick={handleCopy}>
              Copy
            </Button>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={handleDownloadYaml}
            >
              Download
            </Button>
          </Space>
        </div>
        <pre className="bg-gray-50 border border-gray-200 rounded-md p-4 text-xs overflow-auto max-h-64">
          {yamlContent}
        </pre>
      </div>

      <div className="bg-gray-50 border border-gray-200 rounded-md p-4 text-sm flex flex-col gap-1">
        <Title level={5} className="mb-2!">Files</Title>
        <div>
          <Text type="secondary">Template: </Text>
          <Text>{templateFile ? templateFile.name : <Text type="danger">No template uploaded (go back to step 3)</Text>}</Text>
        </div>
        <div>
          <Text type="secondary">Data files: </Text>
          {preparedDataFiles.length > 0 ? (
            <div className="mt-1 flex flex-col gap-1">
              {preparedDataFiles.map((item) => (
                <div key={`${item.viewName}-${item.targetName}`}>
                  {item.file ? (
                    <>
                      <Text>{item.targetName}</Text>
                      {item.sourceName && item.sourceName !== item.targetName && (
                        <Text type="secondary"> {`<- ${item.sourceName}`}</Text>
                      )}
                    </>
                  ) : (
                    <Text type="danger">
                      {`${item.targetName} (missing upload for data source used by ${item.viewName})`}
                    </Text>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <Text type="danger">No data files uploaded (go back to step 1)</Text>
          )}
        </div>
      </div>

      {missingFiles.length > 0 && (
        <Alert
          type="warning"
          message="Missing Data Files"
          description="Some data views do not have an uploaded CSV mapped to them. Go back to Step 1 or Step 2 and fix the mapping before generating."
          showIcon
        />
      )}

      {error && (
        <Alert type="error" message="Generation Failed" description={error} showIcon closable onClose={() => setError(null)} />
      )}

      <Button
        type="primary"
        size="large"
        icon={<SendOutlined />}
        loading={loading}
        disabled={!templateFile || dataFiles.length === 0 || missingFiles.length > 0}
        onClick={handleGenerate}
        block
      >
        Generate Report
      </Button>
    </div>
  );
}
