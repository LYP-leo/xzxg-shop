import { useEffect, useState } from 'react';
import {
  AgentRun,
  AgentTraceEvent,
  DocumentItem,
  EvalRun,
  AppConfig,
  getAdminAgentRunTrace,
  listAccountsPage,
  listAdminAgentRunsPage,
  listAppConfigsPage,
  listAdminOrdersPage,
  listAdminProducts,
  listDocumentsPage,
  listEvalRuns,
  updateAccountStatus,
  updateAppConfig,
  updateAdminProductStatus
} from '../api/admin';
import type { Account } from '../types/auth';
import type { Order } from '../types/order';
import type { ProductCard } from '../types/product';
import { orderStatusText } from './OrderPage';

type AdminPageProps = {
  token: string;
};

export function AdminPage({ token }: AdminPageProps) {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountPage, setAccountPage] = useState(1);
  const [accountPageSize, setAccountPageSize] = useState(10);
  const [accountTotal, setAccountTotal] = useState(0);
  const [accountLoading, setAccountLoading] = useState(false);
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [documentPage, setDocumentPage] = useState(1);
  const [documentPageSize, setDocumentPageSize] = useState(10);
  const [documentTotal, setDocumentTotal] = useState(0);
  const [documentLoading, setDocumentLoading] = useState(false);
  const [evalRuns, setEvalRuns] = useState<EvalRun[]>([]);
  const [products, setProducts] = useState<ProductCard[]>([]);
  const [productPage, setProductPage] = useState(1);
  const [productPageSize, setProductPageSize] = useState(10);
  const [productTotal, setProductTotal] = useState(0);
  const [productLoading, setProductLoading] = useState(false);
  const [orders, setOrders] = useState<Order[]>([]);
  const [orderPage, setOrderPage] = useState(1);
  const [orderPageSize, setOrderPageSize] = useState(10);
  const [orderTotal, setOrderTotal] = useState(0);
  const [orderLoading, setOrderLoading] = useState(false);
  const [configs, setConfigs] = useState<AppConfig[]>([]);
  const [configPage, setConfigPage] = useState(1);
  const [configPageSize, setConfigPageSize] = useState(10);
  const [configTotal, setConfigTotal] = useState(0);
  const [configLoading, setConfigLoading] = useState(false);
  const [configDrafts, setConfigDrafts] = useState<Record<string, string>>({});
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [agentRunPage, setAgentRunPage] = useState(1);
  const [agentRunPageSize, setAgentRunPageSize] = useState(10);
  const [agentRunTotal, setAgentRunTotal] = useState(0);
  const [agentRunLoading, setAgentRunLoading] = useState(false);
  const [selectedRunId, setSelectedRunId] = useState('');
  const [traceEvents, setTraceEvents] = useState<AgentTraceEvent[]>([]);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState('');

  useEffect(() => {
    listEvalRuns().then(setEvalRuns);
  }, [token]);

  useEffect(() => {
    loadAccounts(accountPage, accountPageSize);
  }, [token, accountPage, accountPageSize]);

  useEffect(() => {
    loadDocuments(documentPage, documentPageSize);
  }, [token, documentPage, documentPageSize]);

  useEffect(() => {
    loadProducts(productPage, productPageSize);
  }, [token, productPage, productPageSize]);

  useEffect(() => {
    loadOrders(orderPage, orderPageSize);
  }, [token, orderPage, orderPageSize]);

  useEffect(() => {
    loadConfigs(configPage, configPageSize);
  }, [token, configPage, configPageSize]);

  useEffect(() => {
    loadAgentRuns(agentRunPage, agentRunPageSize);
  }, [token, agentRunPage, agentRunPageSize]);

  async function refresh() {
    await Promise.all([
      loadAccounts(accountPage, accountPageSize),
      loadDocuments(documentPage, documentPageSize),
      loadProducts(productPage, productPageSize),
      loadOrders(orderPage, orderPageSize),
      loadConfigs(configPage, configPageSize),
      loadAgentRuns(agentRunPage, agentRunPageSize)
    ]);
  }

  async function loadAccounts(page: number, pageSize: number) {
    setAccountLoading(true);
    try {
      const data = await listAccountsPage(token, page, pageSize);
      setAccounts(data.items);
      setAccountTotal(data.total);
    } finally {
      setAccountLoading(false);
    }
  }

  async function loadDocuments(page: number, pageSize: number) {
    setDocumentLoading(true);
    try {
      const data = await listDocumentsPage(token, page, pageSize);
      setDocuments(data.items);
      setDocumentTotal(data.total);
    } finally {
      setDocumentLoading(false);
    }
  }

  async function loadProducts(page: number, pageSize: number) {
    setProductLoading(true);
    try {
      const data = await listAdminProducts(token, page, pageSize);
      setProducts(data.items);
      setProductTotal(data.total);
    } finally {
      setProductLoading(false);
    }
  }

  async function loadOrders(page: number, pageSize: number) {
    setOrderLoading(true);
    try {
      const data = await listAdminOrdersPage(token, page, pageSize);
      setOrders(data.items);
      setOrderTotal(data.total);
    } finally {
      setOrderLoading(false);
    }
  }

  async function loadConfigs(page: number, pageSize: number) {
    setConfigLoading(true);
    try {
      const data = await listAppConfigsPage(token, page, pageSize);
      setConfigs(data.items);
      setConfigTotal(data.total);
      setConfigDrafts((current) => ({
        ...current,
        ...Object.fromEntries(data.items.map((item) => [item.config_key, item.config_value]))
      }));
    } finally {
      setConfigLoading(false);
    }
  }

  async function loadAgentRuns(page: number, pageSize: number) {
    setAgentRunLoading(true);
    try {
      const data = await listAdminAgentRunsPage(token, page, pageSize);
      setAgentRuns(data.items);
      setAgentRunTotal(data.total);
    } finally {
      setAgentRunLoading(false);
    }
  }

  async function toggleAccount(account: Account) {
    await updateAccountStatus(token, account.account_id, account.status === 'active' ? 'inactive' : 'active');
    await loadAccounts(accountPage, accountPageSize);
  }

  async function disableProduct(product: ProductCard) {
    await updateAdminProductStatus(token, product.productId, 'inactive');
    await loadProducts(productPage, productPageSize);
  }

  async function saveConfig(config: AppConfig) {
    await updateAppConfig(token, config.config_key, {
      value: configDrafts[config.config_key] ?? '',
      value_type: config.value_type,
      description: config.description,
      is_secret: config.is_secret
    });
    await loadConfigs(configPage, configPageSize);
  }

  async function loadTrace(runId = selectedRunId) {
    const nextRunId = runId.trim();
    if (!nextRunId) {
      setTraceError('请输入 run_id，或从最近请求中选择一条记录');
      return;
    }
    setSelectedRunId(nextRunId);
    setTraceLoading(true);
    setTraceError('');
    try {
      setTraceEvents(await getAdminAgentRunTrace(token, nextRunId));
    } catch (error) {
      setTraceEvents([]);
      setTraceError(error instanceof Error ? error.message : '查询 trace 失败');
    } finally {
      setTraceLoading(false);
    }
  }

  const traceSummary = summarizeTrace(traceEvents);
  const accountTotalPages = totalPages(accountTotal, accountPageSize);
  const documentTotalPages = totalPages(documentTotal, documentPageSize);
  const productTotalPages = Math.max(1, Math.ceil(productTotal / productPageSize));
  const orderTotalPages = totalPages(orderTotal, orderPageSize);
  const configTotalPages = totalPages(configTotal, configPageSize);
  const agentRunTotalPages = totalPages(agentRunTotal, agentRunPageSize);

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>管理与评测</h1>
          <p>用于演示文档状态、知识库构建和评测闭环。</p>
        </div>
      </header>
      <div className="admin-grid">
        <section className="panel">
          <div className="panel-title-row">
            <div>
              <h2>账号</h2>
              <p>
                共 {accountTotal} 个账号，第 {accountPage} / {accountTotalPages} 页
              </p>
            </div>
            <PageSizeSelect
              value={accountPageSize}
              onChange={(value) => {
                setAccountPageSize(value);
                setAccountPage(1);
              }}
            />
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>账号</th>
                <th>角色</th>
                <th>名称</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {accounts.map((account) => (
                <tr key={account.account_id}>
                  <td>{account.username}</td>
                  <td>{account.role}</td>
                  <td>{account.display_name}</td>
                  <td>{account.status}</td>
                  <td>
                    <button className="button button--ghost" onClick={() => toggleAccount(account)}>
                      {account.status === 'active' ? '停用' : '启用'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <PaginationBar
            loading={accountLoading}
            page={accountPage}
            totalPages={accountTotalPages}
            visibleCount={accounts.length}
            total={accountTotal}
            onPrev={() => setAccountPage((current) => Math.max(1, current - 1))}
            onNext={() => setAccountPage((current) => Math.min(accountTotalPages, current + 1))}
          />
        </section>
        <section className="panel">
          <div className="panel-title-row">
            <div>
              <h2>商品</h2>
              <p>
                共 {productTotal} 个商品，第 {productPage} / {productTotalPages} 页
              </p>
            </div>
            <PageSizeSelect
              value={productPageSize}
              onChange={(value) => {
                setProductPageSize(value);
                setProductPage(1);
              }}
            />
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>商品</th>
                <th>商家</th>
                <th>价格</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.productId}>
                  <td>{product.name}</td>
                  <td>{product.merchantName}</td>
                  <td>¥{product.price}</td>
                  <td>
                    <button className="button button--ghost" onClick={() => disableProduct(product)}>
                      下架
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <PaginationBar
            loading={productLoading}
            page={productPage}
            totalPages={productTotalPages}
            visibleCount={products.length}
            total={productTotal}
            onPrev={() => setProductPage((current) => Math.max(1, current - 1))}
            onNext={() => setProductPage((current) => Math.min(productTotalPages, current + 1))}
          />
        </section>
        <section className="panel">
          <div className="panel-title-row">
            <div>
              <h2>订单</h2>
              <p>
                共 {orderTotal} 个订单，第 {orderPage} / {orderTotalPages} 页
              </p>
            </div>
            <PageSizeSelect
              value={orderPageSize}
              onChange={(value) => {
                setOrderPageSize(value);
                setOrderPage(1);
              }}
            />
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>订单</th>
                <th>商家</th>
                <th>状态</th>
                <th>金额</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr key={order.order_id}>
                  <td>{order.order_id}</td>
                  <td>{order.merchant_name}</td>
                  <td>{orderStatusText(order.status)}</td>
                  <td>¥{order.total_amount}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <PaginationBar
            loading={orderLoading}
            page={orderPage}
            totalPages={orderTotalPages}
            visibleCount={orders.length}
            total={orderTotal}
            onPrev={() => setOrderPage((current) => Math.max(1, current - 1))}
            onNext={() => setOrderPage((current) => Math.min(orderTotalPages, current + 1))}
          />
        </section>
        <section className="panel">
          <div className="panel-title-row">
            <div>
              <h2>文档</h2>
              <p>
                共 {documentTotal} 个文档，第 {documentPage} / {documentTotalPages} 页
              </p>
            </div>
            <PageSizeSelect
              value={documentPageSize}
              onChange={(value) => {
                setDocumentPageSize(value);
                setDocumentPage(1);
              }}
            />
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>标题</th>
                <th>类型</th>
                <th>状态</th>
                <th>Chunk</th>
              </tr>
            </thead>
            <tbody>
              {documents.map((document) => (
                <tr key={document.documentId}>
                  <td>{document.title}</td>
                  <td>{document.docType}</td>
                  <td>{document.status}</td>
                  <td>{document.chunkCount}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <PaginationBar
            loading={documentLoading}
            page={documentPage}
            totalPages={documentTotalPages}
            visibleCount={documents.length}
            total={documentTotal}
            onPrev={() => setDocumentPage((current) => Math.max(1, current - 1))}
            onNext={() => setDocumentPage((current) => Math.min(documentTotalPages, current + 1))}
          />
        </section>
        <section className="panel trace-panel">
          <div className="panel-title-row">
            <div>
              <h2>请求全链路追踪</h2>
              <p>
                共 {agentRunTotal} 个请求，第 {agentRunPage} / {agentRunTotalPages} 页。按 run_id 查看 Agent 执行过程。
              </p>
            </div>
            <div className="panel-actions-inline">
              <PageSizeSelect
                value={agentRunPageSize}
                onChange={(value) => {
                  setAgentRunPageSize(value);
                  setAgentRunPage(1);
                }}
              />
              <button className="button button--ghost" onClick={() => loadAgentRuns(agentRunPage, agentRunPageSize)}>
                刷新最近请求
              </button>
            </div>
          </div>
          <div className="trace-toolbar">
            <input
              className="table-input"
              value={selectedRunId}
              placeholder="输入 run_id"
              onChange={(event) => setSelectedRunId(event.target.value)}
            />
            <button className="button" disabled={traceLoading} onClick={() => loadTrace()}>
              {traceLoading ? '查询中' : '查询'}
            </button>
            {traceError ? <span className="trace-error">{traceError}</span> : null}
          </div>
          <div className="trace-summary">
            <div className="trace-summary-item">
              <span>事件数</span>
              <strong>{traceSummary.count}</strong>
            </div>
            <div className="trace-summary-item">
              <span>失败事件</span>
              <strong>{traceSummary.failed}</strong>
            </div>
            <div className="trace-summary-item">
              <span>耗时累计</span>
              <strong>{traceSummary.duration}ms</strong>
            </div>
            <div className="trace-summary-item">
              <span>模型</span>
              <strong>{traceSummary.models || '-'}</strong>
            </div>
          </div>
          <div className="trace-layout">
            <div>
              <h3>最近请求</h3>
              <div className="trace-run-list">
                {agentRuns.map((run) => (
                  <button
                    className={`trace-run-row${selectedRunId === run.run_id ? ' active' : ''}`}
                    key={run.run_id}
                    onClick={() => loadTrace(run.run_id)}
                  >
                    <span>
                      <strong>{run.run_id}</strong>
                      <em>{run.status}</em>
                    </span>
                    <small>账号 {run.account_id || '-'}</small>
                    <small>Trace {run.trace_id || '-'}</small>
                    <small>{formatDateTime(run.created_at)}</small>
                  </button>
                ))}
                {agentRuns.length === 0 ? <p className="empty-text">暂无 Agent 请求记录</p> : null}
              </div>
              <PaginationBar
                loading={agentRunLoading}
                page={agentRunPage}
                totalPages={agentRunTotalPages}
                visibleCount={agentRuns.length}
                total={agentRunTotal}
                onPrev={() => setAgentRunPage((current) => Math.max(1, current - 1))}
                onNext={() => setAgentRunPage((current) => Math.min(agentRunTotalPages, current + 1))}
              />
            </div>
            <div>
              <h3>执行时间线</h3>
              <div className="trace-timeline">
                {traceEvents.map((event, index) => (
                  <article className={`trace-event ${traceStatusClass(event.status)}`} key={event.trace_event_id}>
                    <div className="trace-event-header">
                      <div>
                        <span className="tag">#{index + 1}</span>
                        <strong>{event.stage}</strong>
                        <span>{event.event_type}</span>
                      </div>
                      <time>{formatDateTime(event.created_at)}</time>
                    </div>
                    <div className="trace-event-meta">
                      <span>状态 {event.status}</span>
                      {event.model ? <span>模型 {event.model}</span> : null}
                      {event.duration_ms ? <span>耗时 {event.duration_ms}ms</span> : null}
                      {event.error ? <span className="trace-error">错误 {event.error}</span> : null}
                    </div>
                    {event.stage === 'tools' ? <ToolTraceResult event={event} /> : null}
                    {rawModelOutput(event) ? (
                      <div className="trace-raw-output">
                        <strong>模型原始输出</strong>
                        <pre>{rawModelOutput(event)}</pre>
                      </div>
                    ) : null}
                    {filteredModelOutput(event) ? (
                      <div className="trace-raw-output trace-raw-output--filtered">
                        <strong>前端实际展示文本</strong>
                        <pre>{filteredModelOutput(event)}</pre>
                      </div>
                    ) : null}
                    {event.metadata_json ? <pre className="trace-meta">{formatTraceMetadata(event.metadata_json)}</pre> : null}
                  </article>
                ))}
                {selectedRunId && traceEvents.length === 0 && !traceLoading ? <p className="empty-text">暂无 trace 事件</p> : null}
                {!selectedRunId && !traceLoading ? <p className="empty-text">选择最近请求或输入 run_id 后查看执行过程</p> : null}
              </div>
            </div>
          </div>
        </section>
        <section className="panel">
          <div className="panel-title-row">
            <div>
              <h2>动态配置</h2>
              <p>
                共 {configTotal} 个配置，第 {configPage} / {configTotalPages} 页
              </p>
            </div>
            <PageSizeSelect
              value={configPageSize}
              onChange={(value) => {
                setConfigPageSize(value);
                setConfigPage(1);
              }}
            />
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>配置</th>
                <th>值</th>
                <th>说明</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {configs.map((config) => (
                <tr key={config.config_key}>
                  <td>
                    <strong>{config.config_key}</strong>
                    {config.is_secret ? <span className="tag">secret</span> : null}
                  </td>
                  <td>
                    {config.value_type === 'text' || config.config_key.startsWith('agent.prompt.') ? (
                      <textarea
                        className="table-input table-input--textarea"
                        value={configDrafts[config.config_key] ?? ''}
                        onChange={(event) =>
                          setConfigDrafts((current) => ({ ...current, [config.config_key]: event.target.value }))
                        }
                      />
                    ) : (
                      <input
                        className="table-input"
                        type={config.is_secret ? 'password' : 'text'}
                        value={configDrafts[config.config_key] ?? ''}
                        placeholder={config.is_secret ? '留空不会自动保留旧值，填写后保存' : ''}
                        onChange={(event) =>
                          setConfigDrafts((current) => ({ ...current, [config.config_key]: event.target.value }))
                        }
                      />
                    )}
                  </td>
                  <td>{config.description}</td>
                  <td>
                    <button className="button button--ghost" onClick={() => saveConfig(config)}>
                      保存
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <PaginationBar
            loading={configLoading}
            page={configPage}
            totalPages={configTotalPages}
            visibleCount={configs.length}
            total={configTotal}
            onPrev={() => setConfigPage((current) => Math.max(1, current - 1))}
            onNext={() => setConfigPage((current) => Math.min(configTotalPages, current + 1))}
          />
        </section>
        <section className="panel">
          <h2>评测</h2>
          <table className="data-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>状态</th>
                <th>通过率</th>
              </tr>
            </thead>
            <tbody>
              {evalRuns.map((run) => (
                <tr key={run.evalRunId}>
                  <td>{run.name}</td>
                  <td>{run.status}</td>
                  <td>{run.passRate}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </div>
    </section>
  );
}

function summarizeTrace(events: AgentTraceEvent[]) {
  const models = Array.from(new Set(events.map((event) => event.model).filter(Boolean))).join(', ');
  return {
    count: events.length,
    failed: events.filter((event) => event.status === 'failed' || event.status === 'error').length,
    duration: events.reduce((total, event) => total + (event.duration_ms ?? 0), 0),
    models
  };
}

function PageSizeSelect({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  return (
    <select className="table-input table-input--compact" value={value} onChange={(event) => onChange(Number(event.target.value))}>
      <option value={10}>10 条/页</option>
      <option value={20}>20 条/页</option>
      <option value={50}>50 条/页</option>
    </select>
  );
}

function PaginationBar({
  loading,
  page,
  totalPages,
  visibleCount,
  total,
  onPrev,
  onNext
}: {
  loading: boolean;
  page: number;
  totalPages: number;
  visibleCount: number;
  total: number;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className="pagination-bar">
      <button className="button button--ghost" disabled={loading || page <= 1} onClick={onPrev}>
        上一页
      </button>
      <span>{loading ? '加载中' : `${visibleCount} / ${total}`}</span>
      <button className="button button--ghost" disabled={loading || page >= totalPages} onClick={onNext}>
        下一页
      </button>
    </div>
  );
}

function ToolTraceResult({ event }: { event: AgentTraceEvent }) {
  const metadata = parseTraceMetadata(event.metadata_json);
  if (!metadata) {
    return null;
  }
  const argumentsText = metadata.arguments ? JSON.stringify(metadata.arguments, null, 2) : '';
  const items = traceResultItems(metadata);
  return (
    <div className="trace-tool-result">
      <div className="trace-tool-result__header">
        <strong>工具调用详情</strong>
        <span>{items.length ? `${items.length} 条结果` : '无列表结果'}</span>
      </div>
      {argumentsText ? (
        <details>
          <summary>工具入参</summary>
          <pre>{argumentsText}</pre>
        </details>
      ) : null}
      {event.event_type === 'search_knowledge' && items.length ? (
        <div className="trace-knowledge-list">
          {items.map((item, index) => (
            <article key={`${String(item.chunk_id ?? item.source ?? index)}-${index}`}>
              <div>
                <span className="tag">#{index + 1}</span>
                <strong>{String(item.title ?? item.chunk_id ?? '资料片段')}</strong>
              </div>
              {item.snippet ? <p>{String(item.snippet)}</p> : null}
              <small>
                {item.chunk_id ? `chunk ${String(item.chunk_id)}` : ''}
                {item.source ? ` source ${String(item.source)}` : ''}
              </small>
            </article>
          ))}
        </div>
      ) : items.length ? (
        <pre>{JSON.stringify(items, null, 2)}</pre>
      ) : metadata.result ? (
        <pre>{JSON.stringify(metadata.result, null, 2)}</pre>
      ) : null}
    </div>
  );
}

function traceResultItems(metadata: Record<string, unknown>) {
  const result = metadata.result;
  if (!result || typeof result !== 'object') {
    return [];
  }
  const items = (result as Record<string, unknown>).items;
  if (!Array.isArray(items)) {
    return [];
  }
  return items.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object');
}

function totalPages(total: number, pageSize: number) {
  return Math.max(1, Math.ceil(total / pageSize));
}

function traceStatusClass(status: string) {
  if (status === 'failed' || status === 'error') {
    return 'failed';
  }
  if (status === 'completed' || status === 'success') {
    return 'ok';
  }
  return 'running';
}

function formatTraceMetadata(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function rawModelOutput(event: AgentTraceEvent) {
  const metadata = parseTraceMetadata(event.metadata_json);
  if (!metadata || typeof metadata.raw_output !== 'string') {
    return '';
  }
  return metadata.raw_output;
}

function filteredModelOutput(event: AgentTraceEvent) {
  const metadata = parseTraceMetadata(event.metadata_json);
  if (!metadata || typeof metadata.filtered_output !== 'string') {
    return '';
  }
  return metadata.filtered_output;
}

function parseTraceMetadata(value?: string) {
  if (!value) {
    return null;
  }
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

function formatDateTime(value: string) {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString('zh-CN', { hour12: false });
}
