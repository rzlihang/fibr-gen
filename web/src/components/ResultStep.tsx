import { Result, Button } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  DownloadOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";

interface ResultStepProps {
  loading: boolean;
  error: string | null;
  downloadReady: boolean;
  onDownload: () => void;
  onReset: () => void;
}

export default function ResultStep({
  loading,
  error,
  downloadReady,
  onDownload,
  onReset,
}: ResultStepProps) {
  const { t } = useTranslation();

  if (loading) {
    return (
      <Result
        icon={<LoadingOutlined spin />}
        title={t("result.generating")}
        subTitle={t("result.generatingSubtitle")}
      />
    );
  }

  if (error) {
    return (
      <Result
        status="error"
        icon={<CloseCircleOutlined />}
        title={t("result.failed")}
        subTitle={error}
        extra={
          <Button type="primary" onClick={onReset}>
            {t("result.tryAgain")}
          </Button>
        }
      />
    );
  }

  if (downloadReady) {
    return (
      <Result
        status="success"
        icon={<CheckCircleOutlined />}
        title={t("result.success")}
        extra={[
          <Button type="primary" icon={<DownloadOutlined />} onClick={onDownload} key="download">
            {t("result.downloadReport")}
          </Button>,
          <Button onClick={onReset} key="reset">
            {t("result.generateAnother")}
          </Button>,
        ]}
      />
    );
  }

  return null;
}
