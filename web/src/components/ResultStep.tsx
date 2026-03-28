import { Result, Button } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  DownloadOutlined,
} from "@ant-design/icons";

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
  if (loading) {
    return (
      <Result
        icon={<LoadingOutlined spin />}
        title="Generating Report..."
        subTitle="This may take a moment depending on data size."
      />
    );
  }

  if (error) {
    return (
      <Result
        status="error"
        icon={<CloseCircleOutlined />}
        title="Generation Failed"
        subTitle={error}
        extra={
          <Button type="primary" onClick={onReset}>
            Try Again
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
        title="Report Generated!"
        extra={[
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            onClick={onDownload}
            key="download"
          >
            Download Report
          </Button>,
          <Button onClick={onReset} key="reset">
            Generate Another
          </Button>,
        ]}
      />
    );
  }

  return null;
}
