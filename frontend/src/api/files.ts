import { loadSession } from './auth';

export type UploadedFile = {
  file_id: string;
  object_key: string;
  url: string;
  mime_type: string;
  size_bytes: number;
};

export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append('file', file);
  const response = await fetch('/api/v1/files', {
    method: 'POST',
    headers: authHeaders(),
    body: form
  });
  if (!response.ok) {
    throw new Error(`upload failed: ${response.status}`);
  }
  const payload = (await response.json()) as { file: UploadedFile };
  return payload.file;
}

function authHeaders(): Record<string, string> {
  const token = loadSession()?.token;
  return token ? { Authorization: `Bearer ${token}` } : {};
}
