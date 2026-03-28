import { useState, useCallback } from "react";
import { Steps, Button, message, Alert } from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router";

import UploadStep, { type UploadedFiles } from "../components/UploadStep";
import ReviewStep from "../components/ReviewStep";
import ResultStep from "../components/ResultStep";
import { generateReport, downloadBlob } from "../api/generate";

const initialFiles: UploadedFiles = {
  config: null,
  template: null,
  dataFiles: [],
};

export default function GeneratePage() {
  const navigate = useNavigate();
  const [current, setCurrent] = useState(0);
  const [files, setFiles] = useState<UploadedFiles>(initialFiles);
  const [paramsJson, setParamsJson] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resultBlob, setResultBlob] = useState<Blob | null>(null);

  const canProceedFromUpload =
    files.config !== null &&
    files.template !== null &&
    files.dataFiles.length > 0;

  const isValidParams =
    paramsJson.trim() === "" ||
    (() => {
      try {
        const parsed = JSON.parse(paramsJson);
        return (
          typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)
        );
      } catch {
        return false;
      }
    })();

  const handleGenerate = useCallback(async () => {
    if (!files.config || !files.template || files.dataFiles.length === 0) return;

    setCurrent(2);
    setLoading(true);
    setError(null);
    setResultBlob(null);

    try {
      let params: Record<string, string> | undefined;
      if (paramsJson.trim()) {
        params = JSON.parse(paramsJson);
      }

      const blob = await generateReport({
        config: files.config,
        template: files.template,
        dataFiles: files.dataFiles,
        params,
      });

      setResultBlob(blob);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An unknown error occurred");
    } finally {
      setLoading(false);
    }
  }, [files, paramsJson]);

  const handleDownload = useCallback(() => {
    if (!resultBlob) return;
    const name = files.template?.name.replace(/\.xlsx$/, "") ?? "report";
    downloadBlob(resultBlob, `${name}_output.xlsx`);
    message.success("Download started");
  }, [resultBlob, files.template]);

  const handleReset = useCallback(() => {
    setCurrent(0);
    setFiles(initialFiles);
    setParamsJson("");
    setError(null);
    setResultBlob(null);
  }, []);

  const steps = [
    {
      title: "Upload",
      description: "Template, config & data",
    },
    {
      title: "Review",
      description: "Verify files & params",
    },
    {
      title: "Result",
      description: "Download report",
    },
  ];

  return (
    <div className="max-w-3xl mx-auto p-8">
      <div className="flex items-center gap-2 mb-6">
        <Button
          type="text"
          icon={<ArrowLeftOutlined />}
          onClick={() => navigate("/")}
        />
        <h1 className="text-2xl font-semibold m-0">fibr-gen Report Generator</h1>
      </div>

      <Steps current={current} items={steps} className="mb-8" />

      <Alert
        message="Changes will not be saved"
        description="The files and parameters you upload here are temporary and will not be saved. Download your report before closing this page."
        type="info"
        showIcon
        closable
        className="mb-6"
      />

      <div className="min-h-75">
        {current === 0 && (
          <UploadStep files={files} onChange={setFiles} />
        )}

        {current === 1 && (
          <ReviewStep
            files={files}
            paramsJson={paramsJson}
            onParamsChange={setParamsJson}
          />
        )}

        {current === 2 && (
          <ResultStep
            loading={loading}
            error={error}
            downloadReady={resultBlob !== null}
            onDownload={handleDownload}
            onReset={handleReset}
          />
        )}
      </div>

      {current < 2 && (
        <div className="flex justify-between mt-8">
          <Button
            disabled={current === 0}
            onClick={() => setCurrent((c) => c - 1)}
          >
            Back
          </Button>
          <div className="flex gap-2">
            {current === 0 && (
              <Button
                type="primary"
                disabled={!canProceedFromUpload}
                onClick={() => setCurrent(1)}
              >
                Next
              </Button>
            )}
            {current === 1 && (
              <Button
                type="primary"
                disabled={!isValidParams}
                onClick={handleGenerate}
              >
                Generate Report
              </Button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
