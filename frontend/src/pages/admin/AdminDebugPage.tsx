import { useMemo, useState } from 'react';
import {
  AgentRun,
  AgentTraceEvent,
  VectorIndexStatus,
  getAdminAgentRunTrace,
  getVectorIndexStatus,
  listAdminAgentRunsPage
} from '../../api/admin';
import {
  PageSizeSelect,
  PaginationBar,
  PanelMessage,
  formatCount,
  formatDateTime,
  totalPages,
  useAsyncList,
  useAsyncValue
} from './AdminCommon';

export function AdminDebugPage({ token }: { token: string }) {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [selectedRunId, setSelectedRunId] = useState('');
  const [traceEvents, setTraceEvents] = useState<AgentTraceEvent[]>([]);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState('');
  const [runFilter, setRunFilter] = useState('');
  const runs = useAsyncList<AgentRun>(() => listAdminAgentRunsPage(token, page, pageSize), [token, page, pageSize]);
  const vector = useAsyncValue<VectorIndexStatus>(() => getVectorIndexStatus(token), [token]);
  const pages = totalPages(runs.total, pageSize);
  const visibleRuns = useMemo(() => {
    const keyword = runFilter.trim().toLowerCase();
    if (!keyword) return runs.items;
    return runs.items.filter((run) =>
      [run.run_id, run.trace_id, run.account_id, run.status, run.query_title].some((value) => String(value ?? '').toLowerCase().includes(keyword))
    );
  }, [runs.items, runFilter]);

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

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>调试观测</h1>
          <p>查看 Agent 请求链路、模型输入输出、工具召回和向量库运行状态。</p>
        </div>
      </header>
      <div className="admin-grid">
        <VectorStatusPanel status={vector.value} loading={vector.loading} error={vector.error} onRefresh={vector.reload} />
        <section className="panel trace-panel">
          <div className="panel-title-row">
            <div>
              <h2>请求全链路追踪</h2>
              <p>
                共 {runs.total} 个请求，第 {page} / {pages} 页。按 run_id、用户 Query、状态或 Trace 搜索。
              </p>
            </div>
            <div className="panel-actions-inline">
              <PageSizeSelect
                value={pageSize}
                onChange={(value) => {
                  setPageSize(value);
                  setPage(1);
                }}
              />
              <button className="button button--ghost" disabled={runs.loading} onClick={runs.reload}>
                {runs.loading ? '刷新中' : '刷新最近请求'}
              </button>
            </div>
          </div>
          <div className="trace-toolbar">
            <input className="table-input" value={selectedRunId} placeholder="输入 run_id" onChange={(event) => setSelectedRunId(event.target.value)} />
            <button className="button" disabled={traceLoading} onClick={() => loadTrace()}>
              {traceLoading ? '查询中' : '查询'}
            </button>
            <input className="table-input" value={runFilter} placeholder="筛选最近请求" onChange={(event) => setRunFilter(event.target.value)} />
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
                {visibleRuns.map((run) => (
                  <button className={`trace-run-row${selectedRunId === run.run_id ? ' active' : ''}`} key={run.run_id} onClick={() => loadTrace(run.run_id)}>
                    <strong className="trace-run-title">{run.query_title || '未记录用户 Query'}</strong>
                    <span>
                      <strong>{run.run_id}</strong>
                      <em>{run.status}</em>
                    </span>
                    <small>账号 {run.account_id || '-'}</small>
                    <small>Trace {run.trace_id || '-'}</small>
                    <small>{formatDateTime(run.created_at)}</small>
                  </button>
                ))}
              </div>
              <PanelMessage loading={runs.loading} error={runs.error} empty={!visibleRuns.length ? '暂无 Agent 请求记录' : ''} />
              <PaginationBar
                loading={runs.loading}
                page={page}
                totalPages={pages}
                visibleCount={visibleRuns.length}
                total={runs.total}
                onPrev={() => setPage((current) => Math.max(1, current - 1))}
                onNext={() => setPage((current) => Math.min(pages, current + 1))}
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
                    <TraceSchemaSummary event={event} />
                    {event.stage === 'tools' ? <ToolTraceResult event={event} /> : null}
                    <TraceText title="大模型输入 Prompt" text={modelPrompt(event)} />
                    <TraceText title="模型原始输出" text={rawModelOutput(event)} />
                    <TraceText title="前端实际展示文本" text={filteredModelOutput(event)} filtered />
                    {event.metadata_json ? (
                      <details className="trace-meta-details">
                        <summary>事件 Metadata</summary>
                        <pre className="trace-meta">{formatTraceMetadata(event.metadata_json)}</pre>
                      </details>
                    ) : null}
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

function VectorStatusPanel({
  status,
  loading,
  error,
  onRefresh
}: {
  status?: VectorIndexStatus;
  loading: boolean;
  error?: string;
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
      {error ? <p className="trace-error">状态查询异常：{error}</p> : null}
      {status?.error ? <p className="trace-error">状态查询异常：{status.error}</p> : null}
      <div className="table-scroll">
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
                  <small>
                    {collection.primary_key} / {collection.vector_key}
                  </small>
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
      </div>
      <PanelMessage loading={loading} empty={!status ? '暂无向量库状态' : ''} />
    </section>
  );
}

function ToolTraceResult({ event }: { event: AgentTraceEvent }) {
  const metadata = parseTraceMetadata(event.metadata_json);
  if (!metadata) {
    return null;
  }
  const input = objectValue(metadata.input);
  const output = objectValue(metadata.output);
  const argumentsValue = input?.arguments ?? metadata.arguments;
  const resultValue = output?.result ?? metadata.result;
  const argumentsText = argumentsValue ? JSON.stringify(argumentsValue, null, 2) : '';
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
      ) : resultValue ? (
        <pre>{JSON.stringify(resultValue, null, 2)}</pre>
      ) : null}
    </div>
  );
}

function TraceSchemaSummary({ event }: { event: AgentTraceEvent }) {
  const metadata = parseTraceMetadata(event.metadata_json);
  const summary = objectValue(metadata?.summary);
  const input = objectValue(metadata?.input);
  const output = objectValue(metadata?.output);
  const debug = objectValue(metadata?.debug);
  if (!summary && !input && !output && !debug) {
    return null;
  }
  const chips = traceSummaryChips(summary);
  return (
    <div className="trace-schema-card">
      <div className="trace-schema-card__header">
        <strong>{String(metadata?.event_kind ?? event.event_type)}</strong>
        {metadata?.schema_version ? <span className="tag">{String(metadata.schema_version)}</span> : null}
      </div>
      {chips.length ? (
        <div className="trace-schema-chips">
          {chips.map((chip) => (
            <span key={chip}>{chip}</span>
          ))}
        </div>
      ) : null}
      <div className="trace-schema-sections">
        {input ? <TraceJSONDetails title="结构化输入" value={input} /> : null}
        {output ? <TraceJSONDetails title="结构化输出" value={output} /> : null}
        {debug ? <TraceJSONDetails title="调试信息" value={debug} /> : null}
      </div>
    </div>
  );
}

function TraceJSONDetails({ title, value }: { title: string; value: Record<string, unknown> }) {
  if (!Object.keys(value).length) return null;
  return (
    <details>
      <summary>{title}</summary>
      <pre>{JSON.stringify(value, null, 2)}</pre>
    </details>
  );
}

function TraceText({ title, text, filtered }: { title: string; text: string; filtered?: boolean }) {
  if (!text) return null;
  return (
    <details className={`trace-raw-output${filtered ? ' trace-raw-output--filtered' : ''}`}>
      <summary>
        <strong>{title}</strong>
        <button className="button button--ghost button--tiny" onClick={(event) => copyTraceText(event, text)}>
          复制
        </button>
      </summary>
      <pre>{text}</pre>
    </details>
  );
}

function copyTraceText(event: React.MouseEvent<HTMLButtonElement>, text: string) {
  event.preventDefault();
  event.stopPropagation();
  void navigator.clipboard?.writeText(text);
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

function traceResultItems(metadata: Record<string, unknown>) {
  const output = objectValue(metadata.output);
  const result = output?.result ?? metadata.result;
  if (!result || typeof result !== 'object') {
    return [];
  }
  const items = (result as Record<string, unknown>).items;
  if (!Array.isArray(items)) {
    return [];
  }
  return items.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object');
}

function traceStatusClass(status: string) {
  if (status === 'failed' || status === 'error') return 'failed';
  if (status === 'completed' || status === 'success') return 'ok';
  return 'running';
}

function formatTraceMetadata(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function modelPrompt(event: AgentTraceEvent) {
  const metadata = parseTraceMetadata(event.metadata_json);
  const input = objectValue(metadata?.input);
  const value = input?.messages ?? metadata?.prompt ?? metadata?.input_prompt ?? metadata?.messages ?? metadata?.request;
  if (!value) return '';
  return typeof value === 'string' ? value : JSON.stringify(value, null, 2);
}

function rawModelOutput(event: AgentTraceEvent) {
  const metadata = parseTraceMetadata(event.metadata_json);
  const output = objectValue(metadata?.output);
  const value = output?.raw_output ?? metadata?.raw_output;
  if (typeof value !== 'string') {
    return '';
  }
  return value;
}

function filteredModelOutput(event: AgentTraceEvent) {
  const metadata = parseTraceMetadata(event.metadata_json);
  const output = objectValue(metadata?.output);
  const value = output?.filtered_output ?? metadata?.filtered_output;
  if (typeof value !== 'string') {
    return '';
  }
  return value;
}

function parseTraceMetadata(value?: string) {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function traceSummaryChips(summary?: Record<string, unknown>) {
  if (!summary) return [];
  const chips: string[] = [];
  for (const [label, key] of [
    ['route', 'route'],
    ['intent', 'intent'],
    ['结果', 'relevance_status'],
    ['原因', 'relevance_reason'],
    ['消息', 'message'],
    ['数量', 'result_item_count']
  ]) {
    const value = summary[key];
    if (value !== undefined && value !== null && String(value) !== '') {
      chips.push(`${label}: ${formatChipValue(value)}`);
    }
  }
  for (const [label, key] of [
    ['商品', 'product_ids'],
    ['候选', 'candidate_product_ids'],
    ['剔除', 'dropped_product_ids'],
    ['片段', 'chunk_ids']
  ]) {
    const value = summary[key];
    if (Array.isArray(value) && value.length) {
      chips.push(`${label}: ${value.map(String).join(', ')}`);
    }
  }
  return chips;
}

function formatChipValue(value: unknown) {
  if (Array.isArray(value)) return value.map(String).join(', ');
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}
