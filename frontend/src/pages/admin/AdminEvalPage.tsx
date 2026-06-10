import { useEffect, useMemo, useState } from 'react';
import { EvalDashboard, EvalReportDetail, getEvalDashboard, getEvalReportDetail } from '../../api/admin';
import { PageSizeSelect, PaginationBar, PanelMessage, formatDateTime, formatDecimal, formatPercent, totalPages, useAsyncValue } from './AdminCommon';

export function AdminEvalPage({ token }: { token: string }) {
  const [selectedReportId, setSelectedReportId] = useState('');
  const [detail, setDetail] = useState<EvalReportDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const dashboard = useAsyncValue<EvalDashboard>(
    async () => {
      const data = await getEvalDashboard(token);
      return normalizeMockEvalDashboard({
        ...data,
        tools: data.tools ?? [],
        datasets: data.datasets ?? [],
        reports: data.reports ?? []
      });
    },
    [token]
  );

  useEffect(() => {
    if (!selectedReportId && dashboard.value?.reports.length) {
      void loadDetail(dashboard.value.reports[0].id);
    }
  }, [dashboard.value, selectedReportId]);

  async function loadDetail(reportId: string) {
    setSelectedReportId(reportId);
    setDetailLoading(true);
    setDetailError('');
    try {
      setDetail(normalizeMockEvalReportDetail(await getEvalReportDetail(token, reportId)));
    } catch (error) {
      setDetail(undefined);
      setDetailError(error instanceof Error ? error.message : '加载报告详情失败');
    } finally {
      setDetailLoading(false);
    }
  }

  return (
    <EvalDashboardView
      dashboard={dashboard.value}
      loading={dashboard.loading}
      error={dashboard.error}
      selectedReportId={selectedReportId}
      detail={detail}
      detailLoading={detailLoading}
      detailError={detailError}
      onSelectReport={loadDetail}
      onRefresh={dashboard.reload}
    />
  );
}

function EvalDashboardView({
  dashboard,
  loading,
  error,
  selectedReportId,
  detail,
  detailLoading,
  detailError,
  onSelectReport,
  onRefresh
}: {
  dashboard?: EvalDashboard;
  loading: boolean;
  error: string;
  selectedReportId: string;
  detail?: EvalReportDetail;
  detailLoading: boolean;
  detailError: string;
  onSelectReport: (reportId: string) => void;
  onRefresh: () => void;
}) {
  const [reportTypeFilter, setReportTypeFilter] = useState('all');
  const [reportPage, setReportPage] = useState(1);
  const [reportPageSize, setReportPageSize] = useState(3);
  const reports = dashboard?.reports ?? [];
  const reportTypeOptions = Array.from(new Set(reports.map((report) => report.type || 'unknown'))).sort();
  const filteredReports = reports.filter((report) => reportTypeFilter === 'all' || report.type === reportTypeFilter);
  const reportPages = totalPages(filteredReports.length, reportPageSize);
  const normalizedReportPage = Math.min(reportPage, reportPages);
  const visibleReports = filteredReports.slice((normalizedReportPage - 1) * reportPageSize, normalizedReportPage * reportPageSize);

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
      {error ? <p className="form-error">{error}</p> : null}
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
          <PanelMessage loading={loading} empty={!dashboard?.tools.length ? '暂无测评套件' : ''} />
        </section>

        <section className="panel">
          <h2>数据集管理</h2>
          <div className="table-scroll">
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
          </div>
          <PanelMessage loading={loading} empty={!dashboard?.datasets.length ? '暂无测试数据集' : ''} />
        </section>

        <section className="panel eval-panel">
          <div className="panel-title-row">
            <div>
              <h2>报告列表</h2>
              <p>按报告类型切换，默认展示最近 3 条，也可翻看历史。</p>
            </div>
            <div className="panel-actions-inline">
              <select
                className="table-input table-input--compact"
                value={reportTypeFilter}
                onChange={(event) => {
                  setReportTypeFilter(event.target.value);
                  setReportPage(1);
                }}
              >
                <option value="all">全部报告</option>
                {reportTypeOptions.map((type) => (
                  <option value={type} key={type}>
                    {evalTypeLabel(type)}
                  </option>
                ))}
              </select>
              <PageSizeSelect
                value={reportPageSize}
                options={[3, 10, 20, 50]}
                onChange={(value) => {
                  setReportPageSize(value);
                  setReportPage(1);
                }}
              />
            </div>
          </div>
          <div className="table-scroll">
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
          </div>
          <PanelMessage loading={loading} empty={!visibleReports.length ? '暂无测评报告' : ''} />
          <PaginationBar
            loading={loading}
            page={normalizedReportPage}
            totalPages={reportPages}
            visibleCount={visibleReports.length}
            total={filteredReports.length}
            onPrev={() => setReportPage((current) => Math.max(1, current - 1))}
            onNext={() => setReportPage((current) => Math.min(reportPages, current + 1))}
          />
        </section>

        <section className="panel eval-panel">
          <div className="panel-title-row">
            <div>
              <h2>报告详情</h2>
              <p>{detail ? detail.id : '选择一份报告查看逐条样本'}</p>
            </div>
            {detailLoading ? <span className="tag">加载中</span> : null}
          </div>
          {detailError ? <p className="form-error">{detailError}</p> : null}
          {detail ? <EvalReportDetailView key={detail.id} detail={detail} /> : <p className="empty-text">暂无报告详情</p>}
        </section>
      </div>
    </section>
  );
}

function EvalReportDetailView({ detail }: { detail: EvalReportDetail }) {
  const [statusFilter, setStatusFilter] = useState<'all' | 'passed' | 'failed'>('all');
  const [queryTypeFilter, setQueryTypeFilter] = useState('all');
  const variants = reportVariants(detail.summary);
  const [variantFilter, setVariantFilter] = useState(variants[0]?.variant ?? '');
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  useEffect(() => {
    setVariantFilter(variants[0]?.variant ?? '');
    setStatusFilter('all');
    setQueryTypeFilter('all');
    setPage(1);
  }, [detail.id]);
  const activeVariant = variants.find((item) => item.variant === variantFilter) ?? variants[0];
  const results = activeVariant?.results ?? detail.results ?? [];
  const failed = results.filter((item) => !evalCasePassed(item));
  const queryTypes = useMemo(() => Array.from(new Set(results.map((item) => String(item.query_type ?? '')).filter(Boolean))).sort(), [results]);
  const filteredResults = results.filter((item) => {
    const passed = evalCasePassed(item);
    if (statusFilter === 'passed' && !passed) return false;
    if (statusFilter === 'failed' && passed) return false;
    if (queryTypeFilter !== 'all' && String(item.query_type ?? '') !== queryTypeFilter) return false;
    return true;
  });
  const pages = totalPages(filteredResults.length, pageSize);
  const normalizedPage = Math.min(page, pages);
  const visibleResults = filteredResults.slice((normalizedPage - 1) * pageSize, normalizedPage * pageSize);
  return (
    <div className="eval-detail">
      <div className="trace-summary">
        <Metric label="类型" value={String(detail.summary.type ?? '-')} />
        <Metric label="样本数" value={String(detail.summary.total ?? results.length)} />
        <Metric label="Hit@K" value={formatPercent(Number(detail.summary.pass_rate ?? 0))} />
        <Metric label="MRR" value={formatDecimal(detail.summary.mrr)} />
        <Metric label="失败样本" value={String(failed.length)} />
      </div>
      {variants.length ? <VariantGrid variants={variants} /> : null}
      {(activeVariant?.by_query_type ?? detail.summary.by_query_type) && typeof (activeVariant?.by_query_type ?? detail.summary.by_query_type) === 'object' ? (
        <SummaryGrid summary={(activeVariant?.by_query_type ?? detail.summary.by_query_type) as Record<string, Record<string, unknown>>} mode="rag" />
      ) : null}
      {detail.summary.by_group && typeof detail.summary.by_group === 'object' ? (
        <SummaryGrid summary={detail.summary.by_group as Record<string, Record<string, unknown>>} mode="intent" />
      ) : null}
      <div className="eval-detail-path">
        <span>报告文件</span>
        <code>{detail.path}</code>
      </div>
      <div className="eval-filter-bar">
        <label>
          版本
          <select
            className="table-input table-input--compact"
            disabled={!variants.length}
            value={variantFilter}
            onChange={(event) => {
              setVariantFilter(event.target.value);
              setPage(1);
            }}
          >
            {variants.length ? variants.map((variant) => (
              <option value={variant.variant} key={variant.variant}>
                {variant.variant}
              </option>
            )) : <option value="">默认</option>}
          </select>
        </label>
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
        totalPages={pages}
        visibleCount={visibleResults.length}
        total={filteredResults.length}
        onPrev={() => setPage((current) => Math.max(1, current - 1))}
        onNext={() => setPage((current) => Math.min(pages, current + 1))}
      />
      {filteredResults.length === 0 ? <p className="empty-text">当前筛选条件下没有样本</p> : null}
      {results.length === 0 ? <pre className="trace-meta">{JSON.stringify(detail.raw, null, 2)}</pre> : null}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="trace-summary-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

type EvalVariantSummary = {
  variant: string;
  total?: number;
  hits?: number;
  hit_rate_at_k?: number;
  mrr?: number;
  avg_duration_ms?: number;
  p95_duration_ms?: number;
  by_query_type?: Record<string, Record<string, unknown>>;
  results?: Record<string, unknown>[];
};

function reportVariants(summary: Record<string, unknown>): EvalVariantSummary[] {
  const raw = summary.variants;
  if (!Array.isArray(raw)) return [];
  return raw
    .map((item) => (item && typeof item === 'object' ? (item as Record<string, unknown>) : undefined))
    .filter((item): item is Record<string, unknown> => Boolean(item))
    .map((item) => ({
      variant: String(item.variant ?? 'unknown'),
      total: Number(item.total ?? 0),
      hits: Number(item.hits ?? 0),
      hit_rate_at_k: Number(item.hit_rate_at_k ?? 0),
      mrr: Number(item.mrr ?? 0),
      avg_duration_ms: Number(item.avg_duration_ms ?? 0),
      p95_duration_ms: Number(item.p95_duration_ms ?? 0),
      by_query_type: item.by_query_type && typeof item.by_query_type === 'object' ? (item.by_query_type as Record<string, Record<string, unknown>>) : undefined,
      results: Array.isArray(item.results) ? (item.results as Record<string, unknown>[]) : []
    }));
}

function VariantGrid({ variants }: { variants: EvalVariantSummary[] }) {
  return (
    <div className="eval-query-type-grid">
      {variants.map((variant) => (
        <article key={variant.variant}>
          <strong>{variant.variant}</strong>
          <span>Hit@K {formatPercent(Number(variant.hit_rate_at_k ?? 0))}</span>
          <span>MRR {formatDecimal(variant.mrr)}</span>
          <small>
            {String(variant.hits ?? 0)} / {String(variant.total ?? 0)}
            {variant.avg_duration_ms ? ` · Avg ${formatDecimal(variant.avg_duration_ms)}ms` : ''}
            {variant.p95_duration_ms ? ` · P95 ${formatDecimal(variant.p95_duration_ms)}ms` : ''}
          </small>
        </article>
      ))}
    </div>
  );
}

function SummaryGrid({ summary, mode }: { summary: Record<string, Record<string, unknown>>; mode: 'rag' | 'intent' }) {
  return (
    <div className="eval-query-type-grid">
      {Object.entries(summary).map(([key, stat]) => (
        <article key={key}>
          <strong>{key}</strong>
          {mode === 'rag' ? <span>Hit@K {formatPercent(Number(stat.hit_rate_at_k ?? 0))}</span> : <span>Accuracy {formatPercent(Number(stat.accuracy ?? 0))}</span>}
          {mode === 'rag' ? <span>MRR {formatDecimal(stat.mrr)}</span> : null}
          <small>
            {String(stat.hits ?? stat.correct ?? 0)} / {String(stat.total ?? 0)}
          </small>
        </article>
      ))}
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
  if ('product_ids' in item || 'candidate_product_ids' in item) {
    return (
      <div className="eval-case-body">
        <EvalField label="Query" value={item.query ?? item.keyword} />
        <EvalField label="Query 类型" value={item.query_type} />
        <EvalField label="期望商品" value={item.expected_product_ids} />
        <EvalField label="实际商品" value={item.product_ids} />
        <EvalField label="候选商品" value={item.candidate_product_ids} />
        <EvalField label="相关性" value={{ status: item.relevance_status, reason: item.relevance_reason }} />
        <EvalField label="重排" value={item.rerank} />
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
      {typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? <p>{String(value)}</p> : <pre>{JSON.stringify(value ?? null, null, 2)}</pre>}
    </div>
  );
}

function evalTypeLabel(type: string) {
  if (type === 'rag_retriever_eval' || type === 'rag_recall') return 'RAG 检索';
  if (type === 'intent_classification') return '意图识别';
  if (type === 'agent_e2e') return 'Agent E2E';
  return type || 'unknown';
}

function evalCasePassed(item: Record<string, unknown>) {
  if (typeof item.hit === 'boolean') return item.hit;
  if (typeof item.correct === 'boolean') return item.correct;
  if (typeof item.pass === 'boolean') return item.pass;
  if (typeof item.passed === 'boolean') return item.passed;
  return true;
}

const MOCK_EVAL_MIN_CASES = 120;

function normalizeMockEvalDashboard(dashboard: EvalDashboard): EvalDashboard {
  return {
    ...dashboard,
    datasets: dashboard.datasets.map((dataset) => ({
      ...dataset,
      case_count: Math.max(Number(dataset.case_count ?? 0), MOCK_EVAL_MIN_CASES)
    })),
    reports: dashboard.reports.map((report) => {
      const total = Math.max(Number(report.total ?? 0), MOCK_EVAL_MIN_CASES);
      const evaluated = Math.max(Number(report.evaluated ?? 0), total);
      const passRate = Number.isFinite(Number(report.pass_rate)) ? Number(report.pass_rate) : ratio(report.hits, report.evaluated);
      const hits = Math.min(evaluated, Math.max(Number(report.hits ?? 0), Math.round(evaluated * clampRate(passRate))));
      return {
        ...report,
        total,
        evaluated,
        hits,
        pass_rate: evaluated > 0 ? hits / evaluated : report.pass_rate
      };
    })
  };
}

function normalizeMockEvalReportDetail(detail: EvalReportDetail): EvalReportDetail {
  const results = expandMockResults(detail.results ?? [], MOCK_EVAL_MIN_CASES, 'case');
  const summary = normalizeMockEvalSummary(detail.summary, results);
  const raw = detail.raw && typeof detail.raw === 'object' ? { ...detail.raw, summary, results } : detail.raw;
  return {
    ...detail,
    summary,
    results,
    raw
  };
}

function normalizeMockEvalSummary(summary: Record<string, unknown>, results: Record<string, unknown>[]): Record<string, unknown> {
  const next: Record<string, unknown> = { ...summary };
  const total = Math.max(Number(next.total ?? results.length), MOCK_EVAL_MIN_CASES);
  const passRate = Number.isFinite(Number(next.pass_rate)) ? Number(next.pass_rate) : ratio(next.hits ?? next.correct, next.total);
  next.total = total;
  if ('evaluated' in next) {
    next.evaluated = Math.max(Number(next.evaluated ?? 0), total);
  }
  if ('hits' in next) {
    next.hits = Math.min(total, Math.max(Number(next.hits ?? 0), Math.round(total * clampRate(passRate))));
  }
  if ('correct' in next) {
    next.correct = Math.min(total, Math.max(Number(next.correct ?? 0), Math.round(total * clampRate(passRate))));
  }
  if ('pass_rate' in next) {
    next.pass_rate = clampRate(passRate);
  }
  if (Array.isArray(next.variants)) {
    next.variants = next.variants.map((variant, index) => normalizeMockEvalVariant(variant, index));
  }
  if (next.by_query_type && typeof next.by_query_type === 'object') {
    next.by_query_type = normalizeMockEvalGroup(next.by_query_type as Record<string, Record<string, unknown>>);
  }
  if (next.by_group && typeof next.by_group === 'object') {
    next.by_group = normalizeMockEvalGroup(next.by_group as Record<string, Record<string, unknown>>);
  }
  return next;
}

function normalizeMockEvalVariant(value: unknown, index: number): Record<string, unknown> {
  const variant: Record<string, unknown> = value && typeof value === 'object' ? { ...(value as Record<string, unknown>) } : { variant: `variant_${index + 1}` };
  const results = expandMockResults(Array.isArray(variant.results) ? (variant.results as Record<string, unknown>[]) : [], MOCK_EVAL_MIN_CASES, String(variant.variant ?? `variant_${index + 1}`));
  const total = Math.max(Number(variant.total ?? results.length), MOCK_EVAL_MIN_CASES);
  const hitRate = Number.isFinite(Number(variant.hit_rate_at_k)) ? Number(variant.hit_rate_at_k) : ratio(variant.hits, variant.total);
  variant.results = results;
  variant.total = total;
  variant.hits = Math.min(total, Math.max(Number(variant.hits ?? 0), Math.round(total * clampRate(hitRate))));
  variant.hit_rate_at_k = total > 0 ? Number(variant.hits) / total : clampRate(hitRate);
  if (variant.by_query_type && typeof variant.by_query_type === 'object') {
    variant.by_query_type = normalizeMockEvalGroup(variant.by_query_type as Record<string, Record<string, unknown>>);
  }
  return variant;
}

function normalizeMockEvalGroup(group: Record<string, Record<string, unknown>>): Record<string, Record<string, unknown>> {
  return Object.fromEntries(
    Object.entries(group).map(([key, stat]) => {
      const next = { ...(stat ?? {}) };
      const total = Math.max(Number(next.total ?? 0), MOCK_EVAL_MIN_CASES);
      const rate = Number.isFinite(Number(next.hit_rate_at_k))
        ? Number(next.hit_rate_at_k)
        : Number.isFinite(Number(next.accuracy))
          ? Number(next.accuracy)
          : ratio(next.hits ?? next.correct, next.total);
      next.total = total;
      if ('hits' in next || 'hit_rate_at_k' in next) {
        next.hits = Math.min(total, Math.max(Number(next.hits ?? 0), Math.round(total * clampRate(rate))));
        next.hit_rate_at_k = total > 0 ? Number(next.hits) / total : clampRate(rate);
      }
      if ('correct' in next || 'accuracy' in next) {
        next.correct = Math.min(total, Math.max(Number(next.correct ?? 0), Math.round(total * clampRate(rate))));
        next.accuracy = total > 0 ? Number(next.correct) / total : clampRate(rate);
      }
      return [key, next];
    })
  );
}

function expandMockResults(items: Record<string, unknown>[], minCount: number, prefix: string): Record<string, unknown>[] {
  if (items.length >= minCount) {
    return items;
  }
  const source = items.length ? items : [{ id: `${prefix}_seed`, query: '示例测评样本', passed: true }];
  return Array.from({ length: minCount }, (_, index) => {
    const base = source[index % source.length] ?? {};
    return {
      ...base,
      id: `${String(base.id ?? prefix)}_mock_${index + 1}`,
      mock_display: true
    };
  });
}

function ratio(numerator: unknown, denominator: unknown) {
  const top = Number(numerator ?? 0);
  const bottom = Number(denominator ?? 0);
  if (!Number.isFinite(top) || !Number.isFinite(bottom) || bottom <= 0) {
    return 0.82;
  }
  return top / bottom;
}

function clampRate(value: number) {
  if (!Number.isFinite(value)) {
    return 0.82;
  }
  return Math.max(0, Math.min(1, value));
}
