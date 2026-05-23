import type { Account } from '../types/auth';
import type { Order } from '../types/order';
import type { ProductCard } from '../types/product';
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

export type AppConfig = {
  config_key: string;
  config_value: string;
  value_type: 'string' | 'bool' | 'int' | string;
  description: string;
  is_secret: boolean;
  updated_at: string;
};

export type AgentRun = {
  run_id: string;
  session_id: string;
  message_id: string;
  account_id: string;
  status: string;
  trace_id: string;
  created_at: string;
  updated_at: string;
};

export type AgentTraceEvent = {
  trace_event_id: string;
  run_id: string;
  trace_id: string;
  account_id?: string;
  stage: string;
  event_type: string;
  model?: string;
  status: string;
  duration_ms?: number;
  error?: string;
  metadata_json?: string;
  created_at: string;
};

export type PagedProducts = {
  items: ProductCard[];
  page: number;
  page_size: number;
  total: number;
};

export type PagedResponse<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};

export async function listAccounts(token: string): Promise<Account[]> {
  const data = await requestJSON<{ items: Account[] }>('/admin/accounts', {
    headers: authHeaders(token)
  });
  return data.items;
}

export async function listAccountsPage(token: string, page = 1, pageSize = 10): Promise<PagedResponse<Account>> {
  return requestJSON<PagedResponse<Account>>(`/admin/accounts?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
}

export async function updateAccountStatus(token: string, accountId: string, status: 'active' | 'inactive'): Promise<Account> {
  return requestJSON<Account>(`/admin/accounts/${accountId}`, {
    method: 'PATCH',
    headers: authHeaders(token),
    body: JSON.stringify({ status })
  });
}

export async function listAdminProducts(token: string, page = 1, pageSize = 10): Promise<PagedResponse<ProductCard>> {
  return requestJSON<PagedResponse<ProductCard>>(`/admin/products?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
}

export async function updateAdminProductStatus(token: string, productId: string, status: 'active' | 'inactive' | 'deleted'): Promise<ProductCard> {
  return requestJSON<ProductCard>(`/admin/products/${productId}`, {
    method: 'PATCH',
    headers: authHeaders(token),
    body: JSON.stringify({ status })
  });
}

export async function listAdminOrders(token: string): Promise<Order[]> {
  const data = await requestJSON<{ items: Order[] }>('/admin/orders', {
    headers: authHeaders(token)
  });
  return data.items;
}

export async function listAdminOrdersPage(token: string, page = 1, pageSize = 10): Promise<PagedResponse<Order>> {
  return requestJSON<PagedResponse<Order>>(`/admin/orders?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
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

export async function listDocumentsPage(token: string, page = 1, pageSize = 10): Promise<PagedResponse<DocumentItem>> {
  const data = await requestJSON<PagedResponse<DocumentResponse>>(`/admin/documents?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
  return {
    ...data,
    items: data.items.map((item) => ({
      documentId: item.document_id,
      title: item.title,
      docType: item.doc_type,
      status: item.status,
      chunkCount: item.chunk_count
    }))
  };
}

export async function listAppConfigs(token: string): Promise<AppConfig[]> {
  const data = await requestJSON<{ items: AppConfig[] }>('/admin/configs', {
    headers: authHeaders(token)
  });
  return data.items;
}

export async function listAppConfigsPage(token: string, page = 1, pageSize = 10): Promise<PagedResponse<AppConfig>> {
  return requestJSON<PagedResponse<AppConfig>>(`/admin/configs?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
}

export async function updateAppConfig(token: string, key: string, patch: { value: string; value_type?: string; description?: string; is_secret?: boolean }): Promise<AppConfig> {
  return requestJSON<AppConfig>(`/admin/configs/${encodeURIComponent(key)}`, {
    method: 'PATCH',
    headers: authHeaders(token),
    body: JSON.stringify(patch)
  });
}

export async function listAdminAgentRuns(token: string, limit = 30): Promise<AgentRun[]> {
  const data = await requestJSON<{ items: AgentRun[] }>(`/admin/agent/runs?page=1&page_size=${limit}`, {
    headers: authHeaders(token)
  });
  return data.items;
}

export async function listAdminAgentRunsPage(token: string, page = 1, pageSize = 10): Promise<PagedResponse<AgentRun>> {
  return requestJSON<PagedResponse<AgentRun>>(`/admin/agent/runs?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
}

export async function getAdminAgentRunTrace(token: string, runId: string): Promise<AgentTraceEvent[]> {
  const data = await requestJSON<{ items: AgentTraceEvent[] }>(`/admin/agent/runs/${encodeURIComponent(runId)}/trace`, {
    headers: authHeaders(token)
  });
  return data.items;
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
