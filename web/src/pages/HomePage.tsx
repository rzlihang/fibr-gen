import { Card, Typography } from "antd";
import { UploadOutlined, ToolOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router";
import { useTranslation } from "react-i18next";

const { Title, Text } = Typography;

export default function HomePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  return (
    <div className="max-w-2xl mx-auto p-8">
      <div className="text-center mb-10">
        <Title level={2}>{t("app.title")}</Title>
        <Text type="secondary">{t("app.subtitle")}</Text>
      </div>

      <div className="grid grid-cols-2 gap-6">
        <Card hoverable className="text-center" onClick={() => navigate("/generate")}>
          <UploadOutlined className="text-4xl text-blue-500 mb-4" />
          <Title level={4}>{t("nav.uploadFiles")}</Title>
          <Text type="secondary">{t("nav.uploadFilesDesc")}</Text>
        </Card>

        <Card hoverable className="text-center" onClick={() => navigate("/build")}>
          <ToolOutlined className="text-4xl text-green-500 mb-4" />
          <Title level={4}>{t("nav.buildConfig")}</Title>
          <Text type="secondary">{t("nav.buildConfigDesc")}</Text>
        </Card>
      </div>
    </div>
  );
}
