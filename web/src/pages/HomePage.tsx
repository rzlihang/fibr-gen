import { Card, Typography } from "antd";
import {
  UploadOutlined,
  ToolOutlined,
} from "@ant-design/icons";
import { useNavigate } from "react-router";

const { Title, Text } = Typography;

export default function HomePage() {
  const navigate = useNavigate();

  return (
    <div className="max-w-2xl mx-auto p-8">
      <div className="text-center mb-10">
        <Title level={2}>fibr-gen</Title>
        <Text type="secondary">
          Generate Excel reports from templates and data
        </Text>
      </div>

      <div className="grid grid-cols-2 gap-6">
        <Card
          hoverable
          className="text-center"
          onClick={() => navigate("/generate")}
        >
          <UploadOutlined className="text-4xl text-blue-500 mb-4" />
          <Title level={4}>Upload Files</Title>
          <Text type="secondary">
            Upload your config YAML, Excel template, and CSV data files directly
          </Text>
        </Card>

        <Card
          hoverable
          className="text-center"
          onClick={() => navigate("/build")}
        >
          <ToolOutlined className="text-4xl text-green-500 mb-4" />
          <Title level={4}>Build Config</Title>
          <Text type="secondary">
            Create your configuration interactively with a step-by-step wizard
          </Text>
        </Card>
      </div>
    </div>
  );
}
