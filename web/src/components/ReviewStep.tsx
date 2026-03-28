import { Descriptions, Input, Tag, Typography } from "antd";
import {
  FileExcelOutlined,
  FileTextOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import type { UploadedFiles } from "./UploadStep";

const { Text } = Typography;

interface ReviewStepProps {
  files: UploadedFiles;
  paramsJson: string;
  onParamsChange: (json: string) => void;
}

export default function ReviewStep({
  files,
  paramsJson,
  onParamsChange,
}: ReviewStepProps) {
  const isValidJson =
    paramsJson.trim() === "" || (() => {
      try {
        const parsed = JSON.parse(paramsJson);
        return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed);
      } catch {
        return false;
      }
    })();

  return (
    <div className="flex flex-col gap-6">
      <Descriptions bordered column={1} size="small">
        <Descriptions.Item
          label={
            <span>
              <SettingOutlined /> Config
            </span>
          }
        >
          {files.config?.name ?? <Text type="danger">Not uploaded</Text>}
        </Descriptions.Item>
        <Descriptions.Item
          label={
            <span>
              <FileExcelOutlined /> Template
            </span>
          }
        >
          {files.template?.name ?? <Text type="danger">Not uploaded</Text>}
        </Descriptions.Item>
        <Descriptions.Item
          label={
            <span>
              <FileTextOutlined /> Data Files
            </span>
          }
        >
          {files.dataFiles.length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {files.dataFiles.map((f, i) => (
                <Tag key={i}>{f.name}</Tag>
              ))}
            </div>
          ) : (
            <Text type="danger">No files uploaded</Text>
          )}
        </Descriptions.Item>
      </Descriptions>

      <div>
        <Text strong>Extra Parameters (optional JSON)</Text>
        <Input.TextArea
          className="mt-2"
          rows={3}
          placeholder='{"env": "prod", "archive_date": "2026-03-28"}'
          value={paramsJson}
          onChange={(e) => onParamsChange(e.target.value)}
          status={isValidJson ? undefined : "error"}
        />
        {!isValidJson && (
          <Text type="danger" className="text-xs">
            Must be a valid JSON object
          </Text>
        )}
      </div>
    </div>
  );
}
