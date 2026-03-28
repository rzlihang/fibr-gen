export interface GenerateParams {
  config: File;
  template: File;
  dataFiles: File[];
  params?: Record<string, string>;
}

export interface GenerateError {
  error: string;
}

export async function generateReport(input: GenerateParams): Promise<Blob> {
  const formData = new FormData();
  formData.append("config", input.config);
  formData.append("template", input.template);
  for (const file of input.dataFiles) {
    formData.append("data[]", file);
  }
  if (input.params && Object.keys(input.params).length > 0) {
    formData.append("params", JSON.stringify(input.params));
  }

  const res = await fetch("/api/generate", {
    method: "POST",
    body: formData,
  });

  if (!res.ok) {
    const body = (await res.json()) as GenerateError;
    throw new Error(body.error || `Server error: ${res.status}`);
  }

  return res.blob();
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
