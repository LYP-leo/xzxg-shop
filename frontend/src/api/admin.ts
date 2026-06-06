import type { Account } from '../types/auth';
import type { Order } from '../types/order';
import type { Merchant, ProductCard } from '../types/product';
import { requestJSON } from './http';

export type DocumentItem = {
  documentId: string;
  title: string;
  docType: string;
  status: 'uploaded' | 'parsing' | 'indexing' | 'indexed' | 'failed';
  chunkCount: number;
  sourceUrl?: string;
  contentHash?: string;
};

type DocumentResponse = {
  document_id: string;
  title: string;
  doc_type: string;
  status: DocumentItem['status'];
  chunk_count: number;
  source_url?: string;
  content_hash?: string;
};

export type UnstructuredIngestionInput = {
  merchant_id: string;
  title: string;
  source_type: 'html' | 'json' | 'text';
  source_url?: string;
  content?: string;
  html?: string;
  json_text?: string;
  metadata?: Record<string, unknown>;
  force_reindex?: boolean;
};

export type UnstructuredIngestionResult = {
  document: DocumentResponse;
  duplicate: boolean;
  text_runes: number;
};

export type EvalRun = {
  evalRunId: string;
  name: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  passRate: string;
};

export type EvalDataset = {
  id: string;
  type: string;
  name: string;
  path: string;
  case_count: number;
  updated_at: string;
};

export type EvalReport = {
  id: string;
  type: string;
  dataset: string;
  path: string;
  generated_at: string;
  total: number;
  evaluated: number;
  hits: number;
  pass_rate: number;
};

export type EvalToolSuite = {
  id: string;
  name: string;
  scope: 'tool' | 'agent' | string;
  dataset_id: string;
  latest_report_id: string;
  command: string;
};

export type EvalDashboard = {
  tools: EvalToolSuite[];
  datasets: EvalDataset[];
  reports: EvalReport[];
};

export type EvalReportDetail = {
  id: string;
  path: string;
  summary: Record<string, unknown>;
  results: Record<string, unknown>[];
  raw: Record<string, unknown>;
};

export type AppConfig = {
  config_key: string;
  config_value: string;
  value_type: 'string' | 'bool' | 'int' | string;
  description: string;
  domain?: string;
  is_secret: boolean;
  updated_at: string;
};

export type AgentPrompt = {
  prompt_id: string;
  prompt_key: string;
  title: string;
  content: string;
  status: 'draft' | 'active' | 'archived' | string;
  version: number;
  description: string;
  created_by?: string;
  published_at?: string;
  created_at: string;
  updated_at: string;
};

export type AgentPromptPublishRecord = {
  record_id: string;
  prompt_key: string;
  prompt_id: string;
  version: number;
  published_by?: string;
  publish_target: string;
  created_at: string;
};

export type AgentRun = {
  run_id: string;
  session_id: string;
  message_id: string;
  account_id: string;
  status: string;
  trace_id: string;
  query_title?: string;
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

export type VectorCollectionStatus = {
  name: string;
  kind: string;
  primary_key: string;
  vector_key: string;
  metric_type: string;
  dimension: number;
  row_count: number;
  load_state: string;
};

export type VectorIndexStatus = {
  enabled: boolean;
  ready: boolean;
  address: string;
  collections: VectorCollectionStatus[];
  error?: string;
  updated_at: string;
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

export type AccountStatus = 'active' | 'inactive' | 'risk';
export type ProductStatus = 'active' | 'inactive' | 'deleted' | 'risk';
export type MerchantStatus = 'active' | 'inactive' | 'risk';

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

export async function updateAccountStatus(token: string, accountId: string, status: AccountStatus): Promise<Account> {
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

export async function updateAdminProductStatus(token: string, productId: string, status: ProductStatus): Promise<ProductCard> {
  return requestJSON<ProductCard>(`/admin/products/${productId}`, {
    method: 'PATCH',
    headers: authHeaders(token),
    body: JSON.stringify({ status })
  });
}

export async function listAdminMerchants(token: string, page = 1, pageSize = 10): Promise<PagedResponse<Merchant>> {
  return requestJSON<PagedResponse<Merchant>>(`/admin/merchants?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
}

export async function updateAdminMerchantStatus(token: string, merchantId: string, status: MerchantStatus): Promise<Merchant> {
  return requestJSON<Merchant>(`/admin/merchants/${merchantId}`, {
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
    chunkCount: item.chunk_count,
    sourceUrl: item.source_url,
    contentHash: item.content_hash
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
      chunkCount: item.chunk_count,
      sourceUrl: item.source_url,
      contentHash: item.content_hash
    }))
  };
}

export async function createAdminUnstructuredIngestion(token: string, input: UnstructuredIngestionInput): Promise<UnstructuredIngestionResult> {
  return requestJSON<UnstructuredIngestionResult>('/admin/unstructured-ingestions', {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(input)
  });
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

export async function listAgentPromptsPage(token: string, page = 1, pageSize = 10): Promise<PagedResponse<AgentPrompt> & { publish_records: AgentPromptPublishRecord[] }> {
  return requestJSON<PagedResponse<AgentPrompt> & { publish_records: AgentPromptPublishRecord[] }>(`/admin/prompts?page=${page}&page_size=${pageSize}`, {
    headers: authHeaders(token)
  });
}

export async function saveAgentPromptDraft(
  token: string,
  promptKey: string,
  patch: { title: string; content: string; description?: string }
): Promise<AgentPrompt> {
  return requestJSON<AgentPrompt>(`/admin/prompts/${encodeURIComponent(promptKey)}`, {
    method: 'PATCH',
    headers: authHeaders(token),
    body: JSON.stringify(patch)
  });
}

export async function publishAgentPrompt(token: string, promptKey: string): Promise<{ prompt: AgentPrompt; record: AgentPromptPublishRecord }> {
  return requestJSON<{ prompt: AgentPrompt; record: AgentPromptPublishRecord }>(`/admin/prompts/${encodeURIComponent(promptKey)}/publish`, {
    method: 'POST',
    headers: authHeaders(token)
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

export async function getVectorIndexStatus(token: string): Promise<VectorIndexStatus> {
  return requestJSON<VectorIndexStatus>('/admin/vector/status', {
    headers: authHeaders(token)
  });
}

export async function getEvalDashboard(token: string): Promise<EvalDashboard> {
  return requestJSON<EvalDashboard>('/admin/evals', {
    headers: authHeaders(token)
  });
}

export async function getEvalReportDetail(token: string, reportId: string): Promise<EvalReportDetail> {
  return requestJSON<EvalReportDetail>(`/admin/evals/reports/${reportId}`, {
    headers: authHeaders(token)
  });
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
