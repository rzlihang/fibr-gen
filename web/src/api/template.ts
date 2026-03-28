export interface ParsedSheet {
  name: string;
  rows: string[][];
  maxRow: number;
  maxCol: number;
}

export interface ParseTemplateResponse {
  sheets: ParsedSheet[];
}

export async function parseTemplate(file: File): Promise<ParseTemplateResponse> {
  const formData = new FormData();
  formData.append("template", file);
  const res = await fetch("/api/template/parse", {
    method: "POST",
    body: formData,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Server error: ${res.status}`);
  }
  return res.json();
}
