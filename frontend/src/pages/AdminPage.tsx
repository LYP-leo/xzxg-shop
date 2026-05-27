import { useEffect, useState } from 'react';
import {
  AgentRun,
  AgentTraceEvent,
  AgentPrompt,
  AgentPromptPublishRecord,
  AppConfig,
  DocumentItem,
  EvalDashboard,
  EvalReportDetail,
  VectorIndexStatus,
  getAdminAgentRunTrace,
  getEvalDashboard,
  getEvalReportDetail,
  getVectorIndexStatus,
  listAccountsPage,
  listAgentPromptsPage,
  listAdminAgentRunsPage,
  listAppConfigsPage,
  listAdminOrdersPage,
  listAdminProducts,
  listDocumentsPage,
  publishAgentPrompt,
  saveAgentPromptDraft,
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
  view?: 'platform' | 'debug' | 'eval' | 'prompts';
};

export function AdminPage({ token, view = 'platform' }: AdminPageProps) {
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
  const [prompts, setPrompts] = useState<AgentPrompt[]>([]);
  const [promptRecords, setPromptRecords] = useState<AgentPromptPublishRecord[]>([]);
  const [promptPage, setPromptPage] = useState(1);
  const [promptPageSize, setPromptPageSize] = useState(10);
  const [promptTotal, setPromptTotal] = useState(0);
  const [promptLoading, setPromptLoading] = useState(false);
  const [promptDrafts, setPromptDrafts] = useState<Record<string, string>>({});
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [agentRunPage, setAgentRunPage] = useState(1);
  const [agentRunPageSize, setAgentRunPageSize] = useState(10);
  const [agentRunTotal, setAgentRunTotal] = useState(0);
  const [agentRunLoading, setAgentRunLoading] = useState(false);
  const [selectedRunId, setSelectedRunId] = useState('');
  const [traceEvents, setTraceEvents] = useState<AgentTraceEvent[]>([]);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState('');
  const [vectorStatus, setVectorStatus] = useState<VectorIndexStatus>();
  const [vectorLoading, setVectorLoading] = useState(false);
  const [evalDashboard, setEvalDashboard] = useState<EvalDashboard>();
  const [evalLoading, setEvalLoading] = useState(false);
  const [selectedEvalReportId, setSelectedEvalReportId] = useState('');
  const [evalReportDetail, setEvalReportDetail] = useState<EvalReportDetail>();
  const [evalDetailLoading, setEvalDetailLoading] = useState(false);

  useEffect(() => {
    if (view === 'platform') {
      loadAccounts(accountPage, accountPageSize);
    }
  }, [token, view, accountPage, accountPageSize]);

  useEffect(() => {
    if (view === 'platform') {
      loadDocuments(documentPage, documentPageSize);
    }
  }, [token, view, documentPage, documentPageSize]);

  useEffect(() => {
    if (view === 'platform') {
      loadProducts(productPage, productPageSize);
    }
  }, [token, view, productPage, productPageSize]);

  useEffect(() => {
    if (view === 'platform') {
      loadOrders(orderPage, orderPageSize);
    }
  }, [token, view, orderPage, orderPageSize]);

  useEffect(() => {
    if (view === 'platform') {
      loadConfigs(configPage, configPageSize);
    }
  }, [token, view, configPage, configPageSize]);

  useEffect(() => {
    if (view === 'debug') {
      loadAgentRuns(agentRunPage, agentRunPageSize);
    }
  }, [token, view, agentRunPage, agentRunPageSize]);

  useEffect(() => {
    if (view === 'debug') {
      loadVectorStatus();
    }
  }, [token, view]);

  useEffect(() => {
    if (view === 'eval') {
      loadEvalDashboard();
    }
  }, [token, view]);

  useEffect(() => {
    if (view === 'prompts') {
      loadPrompts(promptPage, promptPageSize);
    }
  }, [token, view, promptPage, promptPageSize]);

  async function refresh() {
    await Promise.all([
      loadAccounts(accountPage, accountPageSize),
      loadDocuments(documentPage, documentPageSize),
      loadProducts(productPage, productPageSize),
      loadOrders(orderPage, orderPageSize),
      loadConfigs(configPage, configPageSize),
      loadAgentRuns(agentRunPage, agentRunPageSize),
      loadPrompts(promptPage, promptPageSize)
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

  async function loadPrompts(page: number, pageSize: number) {
    setPromptLoading(true);
    try {
      const data = await listAgentPromptsPage(token, page, pageSize);
      setPrompts(data.items);
      setPromptTotal(data.total);
      setPromptRecords(data.publish_records ?? []);
      setPromptDrafts((current) => ({
        ...current,
        ...Object.fromEntries(data.items.map((item) => [item.prompt_key, item.content]))
      }));
    } finally {
      setPromptLoading(false);
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

  async function loadVectorStatus() {
    setVectorLoading(true);
    try {
      setVectorStatus(await getVectorIndexStatus(token));
    } finally {
      setVectorLoading(false);
    }
  }

  async function loadEvalDashboard() {
    setEvalLoading(true);
    try {
      const data = await getEvalDashboard(token);
      setEvalDashboard(data);
      if (!selectedEvalReportId && data.reports.length > 0) {
        await loadEvalReportDetail(data.reports[0].id);
      }
    } finally {
      setEvalLoading(false);
    }
  }

  async function loadEvalReportDetail(reportId: string) {
    setSelectedEvalReportId(reportId);
    setEvalDetailLoading(true);
    try {
      setEvalReportDetail(await getEvalReportDetail(token, reportId));
    } finally {
      setEvalDetailLoading(false);
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

  async function savePrompt(prompt: AgentPrompt) {
    await saveAgentPromptDraft(token, prompt.prompt_key, {
      title: prompt.title,
      description: prompt.description,
      content: promptDrafts[prompt.prompt_key] ?? ''
    });
    await loadPrompts(promptPage, promptPageSize);
  }

  async function publishPrompt(prompt: AgentPrompt) {
    await publishAgentPrompt(token, prompt.prompt_key);
    await loadPrompts(promptPage, promptPageSize);
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
  const promptTotalPages = totalPages(promptTotal, promptPageSize);

  if (view === 'prompts') {
    return (
      <section>
        <header className="page-header">
          <div>
            <h1>Prompt 管理</h1>
            <p>编辑 Prompt 草稿，发布后同步到 Nacos 的 xzxg-shop-agent-prompts.json。</p>
          </div>
          <button className="button button--ghost" onClick={() => loadPrompts(promptPage, promptPageSize)}>
            刷新
          </button>
        </header>
        <div className="admin-grid">
          <section className="panel prompt-panel">
            <div className="panel-title-row">
              <div>
                <h2>Agent Prompt</h2>
                <p>
                  共 {promptTotal} 个 Prompt，第 {promptPage} / {promptTotalPages} 页
                </p>
              </div>
              <PageSizeSelect
                value={promptPageSize}
                onChange={(value) => {
                  setPromptPageSize(value);
                  setPromptPage(1);
                }}
              />
            </div>
            <div className="prompt-list">
              {prompts.map((prompt) => (
                <article className="prompt-editor" key={prompt.prompt_key}>
                  <div className="prompt-editor__header">
                    <div>
                      <strong>{prompt.prompt_key}</strong>
                      <span className="tag">{prompt.status}</span>
                      <span className="tag">v{prompt.version}</span>
                    </div>
                    <div className="panel-actions-inline">
                      <button className="button button--ghost" disabled={promptLoading} onClick={() => savePrompt(prompt)}>
                        保存草稿
                      </button>
                      <button className="button" disabled={promptLoading} onClick={() => publishPrompt(prompt)}>
                        发布到 Nacos
                      </button>
                    </div>
                  </div>
                  <p>{prompt.description || prompt.title}</p>
                  <textarea
                    className="prompt-editor__textarea"
                    value={promptDrafts[prompt.prompt_key] ?? ''}
                    onChange={(event) =>
                      setPromptDrafts((current) => ({ ...current, [prompt.prompt_key]: event.target.value }))
                    }
                  />
                  <small>最近更新 {formatDateTime(prompt.updated_at)}</small>
                </article>
              ))}
              {prompts.length === 0 && !promptLoading ? <p className="empty-text">暂无 Prompt</p> : null}
            </div>
            <PaginationBar
              loading={promptLoading}
              page={promptPage}
              totalPages={promptTotalPages}
              visibleCount={prompts.length}
              total={promptTotal}
              onPrev={() => setPromptPage((current) => Math.max(1, current - 1))}
              onNext={() => setPromptPage((current) => Math.min(promptTotalPages, current + 1))}
            />
          </section>
          <section className="panel">
            <div className="panel-title-row">
              <div>
                <h2>最近发布</h2>
                <p>展示最近 20 次 Prompt 发布记录。</p>
              </div>
            </div>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Prompt</th>
                  <th>版本</th>
                  <th>Nacos DataId</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                {promptRecords.map((record) => (
                  <tr key={record.record_id}>
                    <td>{record.prompt_key}</td>
                    <td>v{record.version}</td>
                    <td>{record.nacos_data_id}</td>
                    <td>{formatDateTime(record.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        </div>
      </section>
    );
  }

  if (view === 'debug') {
    return (
      <section>
        <header className="page-header">
          <div>
            <h1>调试观测</h1>
            <p>查看 Agent 请求链路、模型输出、工具召回和向量库运行状态。</p>
          </div>
        </header>
        <div className="admin-grid">
          <VectorStatusPanel status={vectorStatus} loading={vectorLoading} onRefresh={loadVectorStatus} />
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
                      {event.event_type === 'llm_call' ? <LLMTracePrompt event={event} /> : null}
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
        </div>
      </section>
    );
  }

  if (view === 'eval') {
    return (
      <EvalDashboardView
        dashboard={evalDashboard}
        loading={evalLoading}
        selectedReportId={selectedEvalReportId}
        detail={evalReportDetail}
        detailLoading={evalDetailLoading}
        onSelectReport={loadEvalReportDetail}
        onRefresh={loadEvalDashboard}
      />
    );
  }

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
      </div>
    </section>
  );
}

function EvalDashboardView({
  dashboard,
  loading,
  selectedReportId,
  detail,
  detailLoading,
  onSelectReport,
  onRefresh
}: {
  dashboard?: EvalDashboard;
  loading: boolean;
  selectedReportId: string;
  detail?: EvalReportDetail;
  detailLoading: boolean;
  onSelectReport: (reportId: string) => void;
  onRefresh: () => void;
}) {
  const [reportTypeFilter, setReportTypeFilter] = useState('all');
  const reportTypeOptions = Array.from(new Set((dashboard?.reports ?? []).map((report) => report.type || 'unknown'))).sort();
  const visibleReports = (dashboard?.reports ?? []).filter((report) => reportTypeFilter === 'all' || report.type === reportTypeFilter).slice(0, 3);
  return (
    <section>
      <header className="page-header">
        <div>
          <h1>质量评测</h1>
          <p>集中查看 RAG、工具级评测、Agent 端到端评测和测试数据集。</p>
        </div>
        <button className="button button--ghost" disabled={loading} onClick={onRefresh}>
          {loading ? '刷新中' : '刷新'}
        </button>
      </header>
      <div className="admin-grid">
        <section className="panel eval-panel">
          <div className="panel-title-row">
            <div>
              <h2>测评套件</h2>
              <p>每个 tool 独立测，Agent 端到端单独测。</p>
            </div>
          </div>
          <div className="eval-suite-grid">
            {(dashboard?.tools ?? []).map((tool) => {
              const dataset = dashboard?.datasets.find((item) => item.id === tool.dataset_id);
              const report = dashboard?.reports.find((item) => item.id === tool.latest_report_id);
              return (
                <article className="eval-suite-card" key={tool.id}>
                  <div>
                    <span className="tag">{tool.scope === 'agent' ? 'E2E' : 'Tool'}</span>
                    <h3>{tool.name}</h3>
                    <p>{dataset ? `${dataset.case_count} 条测试数据` : '未绑定数据集'}</p>
                  </div>
                  <div className="eval-suite-metrics">
                    <span>最近通过率</span>
                    <strong>{formatPercent(report?.pass_rate)}</strong>
                    <small>{report ? formatDateTime(report.generated_at) : '暂无报告'}</small>
                  </div>
                  <code>{tool.command}</code>
                </article>
              );
            })}
          </div>
        </section>

        <section className="panel">
          <h2>数据集管理</h2>
          <table className="data-table">
            <thead>
              <tr>
                <th>数据集</th>
                <th>类型</th>
                <th>样本数</th>
                <th>更新时间</th>
                <th>路径</th>
              </tr>
            </thead>
            <tbody>
              {(dashboard?.datasets ?? []).map((dataset) => (
                <tr key={dataset.id}>
                  <td>{dataset.name}</td>
                  <td>{dataset.type}</td>
                  <td>{dataset.case_count}</td>
                  <td>{formatDateTime(dataset.updated_at)}</td>
                  <td>
                    <code>{dataset.path}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!loading && !dashboard?.datasets.length ? <p className="empty-text">暂无测试数据集</p> : null}
        </section>

        <section className="panel eval-panel">
          <div className="panel-title-row">
            <div>
              <h2>最近报告</h2>
              <p>按报告类型切换，默认每类视图展示最近 3 条。</p>
            </div>
            <select className="table-input table-input--compact" value={reportTypeFilter} onChange={(event) => setReportTypeFilter(event.target.value)}>
              <option value="all">全部报告</option>
              {reportTypeOptions.map((type) => (
                <option value={type} key={type}>
                  {evalTypeLabel(type)}
                </option>
              ))}
            </select>
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>报告</th>
                <th>类型</th>
                <th>样本</th>
                <th>命中/评测</th>
                <th>通过率</th>
                <th>生成时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {visibleReports.map((report) => (
                <tr className={selectedReportId === report.id ? 'selected-row' : ''} key={report.id}>
                  <td>
                    <strong>{report.id}</strong>
                    <small>{report.path}</small>
                  </td>
                  <td>{report.type}</td>
                  <td>{report.total}</td>
                  <td>
                    {report.hits} / {report.evaluated}
                  </td>
                  <td>{formatPercent(report.pass_rate)}</td>
                  <td>{formatDateTime(report.generated_at)}</td>
                  <td>
                    <button className="button button--ghost" onClick={() => onSelectReport(report.id)}>
                      查看详情
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {((dashboard?.reports ?? []).filter((report) => reportTypeFilter === 'all' || report.type === reportTypeFilter).length) > 3 ? (
            <p className="empty-text">当前类型仅展示最近 3 条报告，历史报告仍可通过测评文件查看。</p>
          ) : null}
          {!loading && !dashboard?.reports.length ? <p className="empty-text">暂无测评报告</p> : null}
        </section>

        <section className="panel eval-panel">
          <div className="panel-title-row">
            <div>
              <h2>报告详情</h2>
              <p>{detail ? detail.id : '选择一份报告查看逐条样本'}</p>
            </div>
            {detailLoading ? <span className="tag">加载中</span> : null}
          </div>
          {detail ? <EvalReportDetailView detail={detail} /> : <p className="empty-text">暂无报告详情</p>}
        </section>
      </div>
    </section>
  );
}

function EvalReportDetailView({ detail }: { detail: EvalReportDetail }) {
  const [statusFilter, setStatusFilter] = useState<'all' | 'passed' | 'failed'>('all');
  const [queryTypeFilter, setQueryTypeFilter] = useState('all');
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  const results = detail.results ?? [];
  const failed = results.filter((item) => !evalCasePassed(item));
  const queryTypes = Array.from(new Set(results.map((item) => String(item.query_type ?? item.case_type ?? '')).filter(Boolean))).sort();
  const filteredResults = results.filter((item) => {
    const passed = evalCasePassed(item);
    if (statusFilter === 'passed' && !passed) return false;
    if (statusFilter === 'failed' && passed) return false;
    if (queryTypeFilter !== 'all' && String(item.query_type ?? item.case_type ?? '') !== queryTypeFilter) return false;
    return true;
  });
  const totalPagesForCases = totalPages(filteredResults.length, pageSize);
  const normalizedPage = Math.min(page, totalPagesForCases);
  const visibleResults = filteredResults.slice((normalizedPage - 1) * pageSize, normalizedPage * pageSize);
  return (
    <div className="eval-detail">
      <div className="trace-summary">
        <div className="trace-summary-item">
          <span>类型</span>
          <strong>{String(detail.summary.type ?? '-')}</strong>
        </div>
        <div className="trace-summary-item">
          <span>样本数</span>
          <strong>{String(detail.summary.total ?? results.length)}</strong>
        </div>
        <div className="trace-summary-item">
          <span>Hit@K</span>
          <strong>{formatPercent(Number(detail.summary.pass_rate ?? 0))}</strong>
        </div>
        <div className="trace-summary-item">
          <span>MRR</span>
          <strong>{formatDecimal(detail.summary.mrr)}</strong>
        </div>
        <div className="trace-summary-item">
          <span>失败样本</span>
          <strong>{failed.length}</strong>
        </div>
        <div className="trace-summary-item">
          <span>P95 耗时</span>
          <strong>{formatDuration(detail.summary.image_total_duration_ms_p95 ?? (detail.summary.latency_ms as Record<string, unknown> | undefined)?.p95)}</strong>
        </div>
      </div>
      {detail.summary.by_query_type && typeof detail.summary.by_query_type === 'object' ? (
        <div className="eval-query-type-grid">
          {Object.entries(detail.summary.by_query_type as Record<string, Record<string, unknown>>).map(([type, stat]) => (
            <article key={type}>
              <strong>{type}</strong>
              <span>Hit@K {formatPercent(Number(stat.hit_rate_at_k ?? 0))}</span>
              <span>MRR {formatDecimal(stat.mrr)}</span>
              <small>
                {String(stat.hits ?? 0)} / {String(stat.total ?? 0)}
              </small>
            </article>
          ))}
        </div>
      ) : null}
      {detail.summary.by_case_type && typeof detail.summary.by_case_type === 'object' ? (
        <div className="eval-query-type-grid">
          {Object.entries(detail.summary.by_case_type as Record<string, Record<string, unknown>>).map(([type, stat]) => (
            <article key={type}>
              <strong>{type}</strong>
              <span>通过率 {formatPercent(Number(stat.pass_rate ?? 0))}</span>
              <span>P95 {formatDuration((stat.latency_ms as Record<string, unknown> | undefined)?.p95)}</span>
              <small>
                {String(stat.hits ?? 0)} / {String(stat.total ?? 0)}
              </small>
            </article>
          ))}
        </div>
      ) : null}
      {detail.summary.by_group && typeof detail.summary.by_group === 'object' ? (
        <div className="eval-query-type-grid">
          {Object.entries(detail.summary.by_group as Record<string, Record<string, unknown>>).map(([group, stat]) => (
            <article key={group}>
              <strong>{group}</strong>
              <span>Accuracy {formatPercent(Number(stat.accuracy ?? 0))}</span>
              <small>
                {String(stat.correct ?? 0)} / {String(stat.total ?? 0)}
              </small>
            </article>
          ))}
        </div>
      ) : null}
      <div className="eval-detail-path">
        <span>报告文件</span>
        <code>{detail.path}</code>
      </div>
      <div className="eval-filter-bar">
        <label>
          结果
          <select
            className="table-input table-input--compact"
            value={statusFilter}
            onChange={(event) => {
              setStatusFilter(event.target.value as 'all' | 'passed' | 'failed');
              setPage(1);
            }}
          >
            <option value="all">全部</option>
            <option value="passed">只看通过</option>
            <option value="failed">只看失败</option>
          </select>
        </label>
        <label>
          Query 类型
          <select
            className="table-input table-input--compact"
            value={queryTypeFilter}
            onChange={(event) => {
              setQueryTypeFilter(event.target.value);
              setPage(1);
            }}
          >
            <option value="all">全部</option>
            {queryTypes.map((type) => (
              <option value={type} key={type}>
                {type}
              </option>
            ))}
          </select>
        </label>
        <label>
          每页
          <PageSizeSelect
            value={pageSize}
            onChange={(value) => {
              setPageSize(value);
              setPage(1);
            }}
          />
        </label>
        <span>
          {filteredResults.length} / {results.length} 条
        </span>
      </div>
      <div className="eval-case-list">
        {visibleResults.map((item, index) => (
          <article className={`eval-case-card ${evalCasePassed(item) ? 'ok' : 'failed'}`} key={String(item.id ?? index)}>
            <div className="eval-case-header">
              <div>
                <span className="tag">#{(normalizedPage - 1) * pageSize + index + 1}</span>
                <strong>{String(item.id ?? 'case')}</strong>
              </div>
              <span>{evalCasePassed(item) ? '通过' : '失败'}</span>
            </div>
            <EvalCaseBody item={item} />
          </article>
        ))}
      </div>
      <PaginationBar
        loading={false}
        page={normalizedPage}
        totalPages={totalPagesForCases}
        visibleCount={visibleResults.length}
        total={filteredResults.length}
        onPrev={() => setPage((current) => Math.max(1, current - 1))}
        onNext={() => setPage((current) => Math.min(totalPagesForCases, current + 1))}
      />
      {filteredResults.length === 0 ? <p className="empty-text">当前筛选条件下没有样本</p> : null}
      {results.length === 0 ? <pre className="trace-meta">{JSON.stringify(detail.raw, null, 2)}</pre> : null}
    </div>
  );
}

function EvalCaseBody({ item }: { item: Record<string, unknown> }) {
  if (Array.isArray(item.recalled)) {
    return (
      <div className="eval-case-body">
        <EvalField label="Query" value={item.keyword ?? item.query} />
        <EvalField label="Query 类型" value={item.query_type} />
        <EvalField label="期望 Chunk" value={item.expected_chunk_ids ?? item.expected_recall} />
        <EvalField label="期望商品" value={item.expected_product_ids ?? item.expected_sources} />
        <EvalField label="命中排名" value={{ best_rank: item.best_rank, product_rank: item.product_rank, chunk_rank: item.chunk_rank }} />
        <div>
          <strong>实际召回</strong>
          <div className="trace-knowledge-list eval-recall-list">
            {item.recalled.map((value, index) => {
              const citation = value && typeof value === 'object' ? (value as Record<string, unknown>) : {};
              return (
                <article key={`${String(citation.chunkId ?? citation.chunk_id ?? index)}-${index}`}>
                  <div>
                    <span className="tag">#{index + 1}</span>
                    <strong>{String(citation.title ?? citation.chunkId ?? citation.chunk_id ?? '召回片段')}</strong>
                  </div>
                  {citation.snippet ? <p>{String(citation.snippet)}</p> : null}
                  <small>
                    {citation.chunkId || citation.chunk_id ? `chunk ${String(citation.chunkId ?? citation.chunk_id)}` : ''}
                    {citation.source ? ` source ${String(citation.source)}` : ''}
                  </small>
                </article>
              );
            })}
          </div>
        </div>
      </div>
    );
  }
  if ('answer' in item || 'blocks' in item) {
    return (
      <div className="eval-case-body">
        <EvalField label="Query" value={item.query} />
        <EvalField label="回答" value={item.answer} />
        <EvalField label="结构化块" value={item.blocks} />
        <EvalField label="Trace" value={[item.run_id, item.trace_id].filter(Boolean)} />
      </div>
    );
  }
  if ('actual_status' in item || 'match_status' in item) {
    return (
      <div className="eval-case-body">
        <EvalField label="Case 类型" value={item.case_type} />
        <EvalField label="期望状态" value={item.expected_status} />
        <EvalField label="实际状态" value={item.actual_status} />
        <EvalField label="匹配状态" value={item.match_status} />
        <EvalField label="处理耗时" value={item.durations ?? { latency_ms: item.latency_ms }} />
      </div>
    );
  }
  return (
    <div className="eval-case-body">
      <EvalField label="Query" value={item.query ?? item.keyword} />
      <EvalField label="期望" value={item.expected_intent ?? item.expected} />
      <EvalField label="实际" value={item.intent ?? item.actual} />
      <EvalField label="原始数据" value={item} />
    </div>
  );
}

function EvalField({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="eval-field">
      <strong>{label}</strong>
      {typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? (
        <p>{String(value)}</p>
      ) : (
        <pre>{JSON.stringify(value ?? null, null, 2)}</pre>
      )}
    </div>
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

function VectorStatusPanel({
  status,
  loading,
  onRefresh
}: {
  status?: VectorIndexStatus;
  loading: boolean;
  onRefresh: () => void;
}) {
  const productCollection = status?.collections.find((item) => item.kind === 'products');
  const knowledgeCollection = status?.collections.find((item) => item.kind === 'knowledge');
  return (
    <section className="panel vector-panel">
      <div className="panel-title-row">
        <div>
          <h2>向量数据库</h2>
          <p>Milvus 商品向量与知识片段向量索引状态。</p>
        </div>
        <button className="button button--ghost" disabled={loading} onClick={onRefresh}>
          {loading ? '刷新中' : '刷新'}
        </button>
      </div>
      <div className="trace-summary vector-summary">
        <div className="trace-summary-item">
          <span>服务</span>
          <strong>{status?.enabled ? '已启用' : '未启用'}</strong>
        </div>
        <div className="trace-summary-item">
          <span>索引状态</span>
          <strong>{status?.ready ? 'ready' : 'initializing'}</strong>
        </div>
        <div className="trace-summary-item">
          <span>商品向量</span>
          <strong>{formatCount(productCollection?.row_count)}</strong>
        </div>
        <div className="trace-summary-item">
          <span>知识片段</span>
          <strong>{formatCount(knowledgeCollection?.row_count)}</strong>
        </div>
      </div>
      <div className="vector-address">
        <span>Milvus</span>
        <strong>{status?.address || '-'}</strong>
      </div>
      {status?.error ? <p className="trace-error">状态查询异常：{status.error}</p> : null}
      <table className="data-table vector-table">
        <thead>
          <tr>
            <th>Collection</th>
            <th>用途</th>
            <th>行数</th>
            <th>维度</th>
            <th>Metric</th>
            <th>Load</th>
          </tr>
        </thead>
        <tbody>
          {(status?.collections ?? []).map((collection) => (
            <tr key={collection.name}>
              <td>
                <strong>{collection.name}</strong>
                <small>{collection.primary_key} / {collection.vector_key}</small>
              </td>
              <td>{collection.kind === 'products' ? '商品召回' : '知识召回'}</td>
              <td>{formatCount(collection.row_count)}</td>
              <td>{collection.dimension || '-'}</td>
              <td>{collection.metric_type || '-'}</td>
              <td>{collection.load_state || '-'}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {!status && !loading ? <p className="empty-text">暂无向量库状态</p> : null}
    </section>
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

function LLMTracePrompt({ event }: { event: AgentTraceEvent }) {
  const metadata = parseTraceMetadata(event.metadata_json);
  if (!metadata) {
    return null;
  }
  const messages = Array.isArray(metadata.messages)
    ? metadata.messages.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object')
    : [];
  if (!messages.length) {
    return null;
  }
  const promptChars = typeof metadata.prompt_chars === 'number' ? metadata.prompt_chars : undefined;
  const promptHash = typeof metadata.prompt_hash === 'string' ? metadata.prompt_hash : '';
  const temperature = typeof metadata.temperature === 'number' ? metadata.temperature : undefined;
  return (
    <div className="trace-raw-output trace-raw-output--prompt">
      <details>
        <summary>
          大模型输入 Prompt
          {promptChars !== undefined ? ` · ${promptChars} 字` : ''}
          {temperature !== undefined ? ` · temperature ${temperature}` : ''}
        </summary>
        {promptHash ? <small>hash {promptHash}</small> : null}
        <div className="trace-prompt-messages">
          {messages.map((message, index) => (
            <article key={`${String(message.role ?? 'message')}-${index}`}>
              <strong>{String(message.role ?? 'unknown')}</strong>
              <pre>{String(message.content ?? '')}</pre>
            </article>
          ))}
        </div>
      </details>
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

function formatCount(value?: number) {
  if (value === undefined || value === null) {
    return '-';
  }
  return value.toLocaleString('zh-CN');
}

function formatPercent(value?: number) {
  if (value === undefined || value === null) {
    return '-';
  }
  return `${Math.round(value * 1000) / 10}%`;
}

function evalTypeLabel(type: string) {
  if (type === 'rag_retriever_eval' || type === 'rag_recall') return 'RAG 检索';
  if (type === 'image_search_eval') return '图片搜索';
  if (type === 'intent_classification') return '意图识别';
  if (type === 'agent_e2e') return 'Agent E2E';
  return type || 'unknown';
}

function formatDuration(value: unknown) {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return '-';
  }
  return `${Math.round(number)}ms`;
}

function formatDecimal(value: unknown) {
  const number = Number(value);
  if (Number.isNaN(number)) {
    return '-';
  }
  return String(Math.round(number * 1000) / 1000);
}

function evalCasePassed(item: Record<string, unknown>) {
  if (typeof item.hit === 'boolean') {
    return item.hit;
  }
  if (typeof item.correct === 'boolean') {
    return item.correct;
  }
  if (typeof item.pass === 'boolean') {
    return item.pass;
  }
  if (typeof item.passed === 'boolean') {
    return item.passed;
  }
  return true;
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
