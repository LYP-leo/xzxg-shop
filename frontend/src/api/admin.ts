import type { Account } from '../types/auth';
import { requestJSON } from './http';

export type DocumentItem = {
  documentId: string;
  title: string;
  docType: string;
  status: 'uploaded' | 'parsing' | 'indexing' | 'indexed' | 'failed';
  chunkCount: number;
};

type DocumentResponse = {
  document_id: string;
  title: string;
  doc_type: string;
  status: DocumentItem['status'];
  chunk_count: number;
};

export type EvalRun = {
  evalRunId: string;
  name: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  passRate: string;
};

export async function listAccounts(token: string): Promise<Account[]> {
  const data = await requestJSON<{ items: Account[] }>('/admin/accounts', {
    headers: authHeaders(token)
  });
  return data.items;
}

export async function listDocuments(token: string): Promise<DocumentItem[]> {
  const data = await requestJSON<{ items: DocumentResponse[] }>('/admin/documents', {
    headers: authHeaders(token)
  });
  return data.items.map((item) => ({
    documentId: item.document_id,
    title: item.title,
    docType: item.doc_type,
    status: item.status,
    chunkCount: item.chunk_count
  }));
}

export async function listEvalRuns(): Promise<EvalRun[]> {
  return [
    {
      evalRunId: 'eval_001',
      name: '手机推荐核心集',
      status: 'completed',
      passRate: '86%'
    }
  ];
}

function authHeaders(token: string) {
  return {
    Authorization: `Bearer ${token}`
  };
}
