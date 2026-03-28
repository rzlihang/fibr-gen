import { Upload, Typography } from "antd";
import {
  FileExcelOutlined,
  FileTextOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import type { UploadFile } from "antd";

const { Dragger } = Upload;
const { Title, Text } = Typography;

export interface UploadedFiles {
  config: File | null;
  template: File | null;
  dataFiles: File[];
}

interface UploadStepProps {
  files: UploadedFiles;
  onChange: (files: UploadedFiles) => void;
}

export default function UploadStep({ files, onChange }: UploadStepProps) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div>
        <Title level={5} className="m-0 mb-3">
          Config Bundle (.yaml)
        </Title>
        <Dragger
          accept=".yaml,.yml"
          maxCount={1}
          fileList={
            files.config
              ? [
                  {
                    uid: "config",
                    name: files.config.name,
                    status: "done",
                  } as UploadFile,
                ]
              : []
          }
          beforeUpload={(file) => {
            onChange({ ...files, config: file });
            return false;
          }}
          onRemove={() => {
            onChange({ ...files, config: null });
          }}
          className="h-full"
        >
          <p className="text-3xl text-gray-400">
            <SettingOutlined />
          </p>
          <Text>Click or drag your config YAML file here</Text>
        </Dragger>
      </div>

      <div>
        <Title level={5} className="m-0 mb-3">
          Template (.xlsx)
        </Title>
        <Dragger
          accept=".xlsx"
          maxCount={1}
          fileList={
            files.template
              ? [
                  {
                    uid: "template",
                    name: files.template.name,
                    status: "done",
                  } as UploadFile,
                ]
              : []
          }
          beforeUpload={(file) => {
            onChange({ ...files, template: file });
            return false;
          }}
          onRemove={() => {
            onChange({ ...files, template: null });
          }}
          className="h-full"
        >
          <p className="text-3xl text-gray-400">
            <FileExcelOutlined />
          </p>
          <Text>Click or drag your Excel template here</Text>
        </Dragger>
      </div>

      <div>
        <Title level={5} className="m-0 mb-3">
          Data Files (.csv)
        </Title>
        <Dragger
          accept=".csv"
          multiple
          fileList={files.dataFiles.map((f, i) => ({
            uid: String(i),
            name: f.name,
            status: "done" as const,
          }))}
          beforeUpload={(file) => {
            onChange({ ...files, dataFiles: [...files.dataFiles, file] });
            return false;
          }}
          onRemove={(file) => {
            const idx = Number(file.uid);
            const next = files.dataFiles.filter((_, i) => i !== idx);
            onChange({ ...files, dataFiles: next });
          }}
          className="h-full"
        >
          <p className="text-3xl text-gray-400">
            <FileTextOutlined />
          </p>
          <Text>Click or drag CSV data files here (one per data view)</Text>
        </Dragger>
      </div>
    </div>
  );
}
