import { Descriptions, Input, Tag, Typography } from "antd";
import { FileExcelOutlined, FileTextOutlined, SettingOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { UploadedFiles } from "./UploadStep";

const { Text } = Typography;

interface ReviewStepProps {
  files: UploadedFiles;
  paramsJson: string;
  onParamsChange: (json: string) => void;
}

export default function ReviewStep({ files, paramsJson, onParamsChange }: ReviewStepProps) {
  const { t } = useTranslation();

  const isValidJson =
    paramsJson.trim() === "" ||
    (() => {
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
              <SettingOutlined /> {t("review.config")}
            </span>
          }
        >
          {files.config?.name ?? <Text type="danger">{t("review.notUploaded")}</Text>}
        </Descriptions.Item>
        <Descriptions.Item
          label={
            <span>
              <FileExcelOutlined /> {t("review.template")}
            </span>
          }
        >
          {files.template?.name ?? <Text type="danger">{t("review.notUploaded")}</Text>}
        </Descriptions.Item>
        <Descriptions.Item
          label={
            <span>
              <FileTextOutlined /> {t("review.dataFiles")}
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
            <Text type="danger">{t("review.noFilesUploaded")}</Text>
          )}
        </Descriptions.Item>
      </Descriptions>

      <div>
        <Text strong>{t("review.extraParams")}</Text>
        <Input.TextArea
          className="mt-2"
          rows={3}
          placeholder={t("review.extraParamsPlaceholder")}
          value={paramsJson}
          onChange={(e) => onParamsChange(e.target.value)}
          status={isValidJson ? undefined : "error"}
        />
        {!isValidJson && (
          <Text type="danger" className="text-xs">
            {t("review.invalidJson")}
          </Text>
        )}
      </div>
    </div>
  );
}
