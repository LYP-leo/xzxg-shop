import { useMemo, useState } from 'react';
import {
  AppConfig,
  DocumentItem,
  UnstructuredIngestionResult,
  createAdminUnstructuredIngestion,
  listAccountsPage,
  listAdminOrdersPage,
  listAdminProducts,
  listAppConfigsPage,
  listDocumentsPage,
  updateAccountStatus,
  updateAdminProductStatus,
  updateAppConfig
} from '../../api/admin';
import type { Account } from '../../types/auth';
import type { Order } from '../../types/order';
import type { ProductCard } from '../../types/product';
import { orderStatusText } from '../OrderPage';
import {
  PageSizeSelect,
  PaginationBar,
  PanelMessage,
  formatDateTime,
  productStatus,
  shortURL,
  totalPages,
  useAsyncList,
  useNotice
} from './AdminCommon';

type PlatformTab = 'overview' | 'catalog' | 'orders' | 'knowledge' | 'config';

export function AdminPlatformPage({ token }: { token: string }) {
  const [tab, setTab] = useState<PlatformTab>('overview');
  const [accountPage, setAccountPage] = useState(1);
  const [accountPageSize, setAccountPageSize] = useState(10);
  const [productPage, setProductPage] = useState(1);
  const [productPageSize, setProductPageSize] = useState(10);
  const [orderPage, setOrderPage] = useState(1);
  const [orderPageSize, setOrderPageSize] = useState(10);
  const [documentPage, setDocumentPage] = useState(1);
  const [documentPageSize, setDocumentPageSize] = useState(10);
  const [configPage, setConfigPage] = useState(1);
  const [configPageSize, setConfigPageSize] = useState(10);
  const [savingKey, setSavingKey] = useState('');
  const [mutatingId, setMutatingId] = useState('');
  const [configDrafts, setConfigDrafts] = useState<Record<string, string>>({});
  const [secretEditKeys, setSecretEditKeys] = useState<Record<string, boolean>>({});
  const notice = useNotice();

  const accounts = useAsyncList<Account>(
    () => listAccountsPage(token, accountPage, accountPageSize),
    [token, accountPage, accountPageSize],
    tab === 'overview'
  );
  const products = useAsyncList<ProductCard>(
    () => listAdminProducts(token, productPage, productPageSize),
    [token, productPage, productPageSize],
    tab === 'catalog' || tab === 'overview'
  );
  const orders = useAsyncList<Order>(
    () => listAdminOrdersPage(token, orderPage, orderPageSize),
    [token, orderPage, orderPageSize],
    tab === 'orders' || tab === 'overview'
  );
  const documents = useAsyncList<DocumentItem>(
    () => listDocumentsPage(token, documentPage, documentPageSize),
    [token, documentPage, documentPageSize],
    tab === 'knowledge' || tab === 'overview'
  );
  const configs = useAsyncList<AppConfig>(
    async () => {
      const data = await listAppConfigsPage(token, configPage, configPageSize);
      setConfigDrafts((current) => ({
        ...current,
        ...Object.fromEntries(data.items.map((item) => [item.config_key, item.is_secret ? '' : item.config_value]))
      }));
      return data;
    },
    [token, configPage, configPageSize],
    tab === 'config'
  );

  async function toggleAccount(account: Account) {
    setMutatingId(account.account_id);
    notice.clear();
    try {
      await updateAccountStatus(token, account.account_id, account.status === 'active' ? 'inactive' : 'active');
      notice.showNotice('账号状态已更新');
      await accounts.reload();
    } catch (error) {
      notice.showError(error);
    } finally {
      setMutatingId('');
    }
  }

  async function disableProduct(product: ProductCard) {
    setMutatingId(product.productId);
    notice.clear();
    try {
      await updateAdminProductStatus(token, product.productId, 'inactive');
      notice.showNotice('商品已下架');
      await products.reload();
    } catch (error) {
      notice.showError(error);
    } finally {
      setMutatingId('');
    }
  }

  async function saveConfig(config: AppConfig) {
    const value = configDrafts[config.config_key] ?? '';
    const validationError = validateConfigValue(config, value, Boolean(secretEditKeys[config.config_key]));
    if (validationError) {
      notice.showError(validationError);
      return;
    }
    setSavingKey(config.config_key);
    notice.clear();
    try {
      await updateAppConfig(token, config.config_key, {
        value,
        value_type: config.value_type,
        description: config.description,
        is_secret: config.is_secret
      });
      notice.showNotice('配置已保存');
      setSecretEditKeys((current) => ({ ...current, [config.config_key]: false }));
      await configs.reload();
    } catch (error) {
      notice.showError(error);
    } finally {
      setSavingKey('');
    }
  }

  const accountTotalPages = totalPages(accounts.total, accountPageSize);
  const productTotalPages = totalPages(products.total, productPageSize);
  const orderTotalPages = totalPages(orders.total, orderPageSize);
  const documentTotalPages = totalPages(documents.total, documentPageSize);
  const configTotalPages = totalPages(configs.total, configPageSize);

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>平台管理</h1>
          <p>按业务域管理账号、商品、订单、知识入库和动态配置。</p>
        </div>
        <button className="button button--ghost" onClick={() => reloadActiveTab(tab, { accounts, products, orders, documents, configs })}>
          刷新当前页
        </button>
      </header>
      <div className="admin-tabs">
        {[
          ['overview', '概览'],
          ['catalog', '商品'],
          ['orders', '订单'],
          ['knowledge', '知识入库'],
          ['config', '动态配置']
        ].map(([key, label]) => (
          <button className={tab === key ? 'active' : ''} key={key} onClick={() => setTab(key as PlatformTab)}>
            {label}
          </button>
        ))}
      </div>
      {notice.notice ? <p className="notice">{notice.notice}</p> : null}
      {notice.error ? <p className="form-error">{notice.error}</p> : null}

      {tab === 'overview' ? (
        <div className="risk-summary-grid">
          <SummaryCard title="账号" value={accounts.total} loading={accounts.loading} />
          <SummaryCard title="商品" value={products.total} loading={products.loading} />
          <SummaryCard title="订单" value={orders.total} loading={orders.loading} />
          <SummaryCard title="知识文档" value={documents.total} loading={documents.loading} />
        </div>
      ) : null}

      {tab === 'overview' || tab === 'catalog' ? (
        <section className="panel">
          <PanelHeader
            title="商品"
            subtitle={`共 ${products.total} 个商品，第 ${productPage} / ${productTotalPages} 页`}
            pageSize={productPageSize}
            setPageSize={(value) => {
              setProductPageSize(value);
              setProductPage(1);
            }}
          />
          <div className="table-scroll">
            <table className="data-table">
              <thead>
                <tr>
                  <th>商品</th>
                  <th>商家</th>
                  <th>价格</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {products.items.map((product) => (
                  <tr key={product.productId}>
                    <td>{product.name}</td>
                    <td>{product.merchantName}</td>
                    <td>¥{product.price}</td>
                    <td>{productStatus(product)}</td>
                    <td>
                      <button className="button button--ghost" disabled={mutatingId === product.productId} onClick={() => disableProduct(product)}>
                        {mutatingId === product.productId ? '处理中' : '下架'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <PanelMessage loading={products.loading} error={products.error} empty={!products.items.length ? '暂无商品' : ''} />
          <PaginationBar
            loading={products.loading}
            page={productPage}
            totalPages={productTotalPages}
            visibleCount={products.items.length}
            total={products.total}
            onPrev={() => setProductPage((current) => Math.max(1, current - 1))}
            onNext={() => setProductPage((current) => Math.min(productTotalPages, current + 1))}
          />
        </section>
      ) : null}

      {tab === 'overview' ? (
        <section className="panel">
          <PanelHeader
            title="账号"
            subtitle={`共 ${accounts.total} 个账号，第 ${accountPage} / ${accountTotalPages} 页`}
            pageSize={accountPageSize}
            setPageSize={(value) => {
              setAccountPageSize(value);
              setAccountPage(1);
            }}
          />
          <div className="table-scroll">
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
                {accounts.items.map((account) => (
                  <tr key={account.account_id}>
                    <td>{account.username}</td>
                    <td>{account.role}</td>
                    <td>{account.display_name}</td>
                    <td>{account.status}</td>
                    <td>
                      <button className="button button--ghost" disabled={mutatingId === account.account_id} onClick={() => toggleAccount(account)}>
                        {mutatingId === account.account_id ? '处理中' : account.status === 'active' ? '停用' : '启用'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <PanelMessage loading={accounts.loading} error={accounts.error} empty={!accounts.items.length ? '暂无账号' : ''} />
          <PaginationBar
            loading={accounts.loading}
            page={accountPage}
            totalPages={accountTotalPages}
            visibleCount={accounts.items.length}
            total={accounts.total}
            onPrev={() => setAccountPage((current) => Math.max(1, current - 1))}
            onNext={() => setAccountPage((current) => Math.min(accountTotalPages, current + 1))}
          />
        </section>
      ) : null}

      {tab === 'overview' || tab === 'orders' ? (
        <section className="panel">
          <PanelHeader
            title="订单"
            subtitle={`共 ${orders.total} 个订单，第 ${orderPage} / ${orderTotalPages} 页`}
            pageSize={orderPageSize}
            setPageSize={(value) => {
              setOrderPageSize(value);
              setOrderPage(1);
            }}
          />
          <div className="table-scroll">
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
                {orders.items.map((order) => (
                  <tr key={order.order_id}>
                    <td>{order.order_id}</td>
                    <td>{order.merchant_name}</td>
                    <td>{orderStatusText(order.status)}</td>
                    <td>¥{order.total_amount}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <PanelMessage loading={orders.loading} error={orders.error} empty={!orders.items.length ? '暂无订单' : ''} />
          <PaginationBar
            loading={orders.loading}
            page={orderPage}
            totalPages={orderTotalPages}
            visibleCount={orders.items.length}
            total={orders.total}
            onPrev={() => setOrderPage((current) => Math.max(1, current - 1))}
            onNext={() => setOrderPage((current) => Math.min(orderTotalPages, current + 1))}
          />
        </section>
      ) : null}

      {tab === 'overview' || tab === 'knowledge' ? (
        <KnowledgeSection
          token={token}
          documents={documents}
          documentPage={documentPage}
          documentPageSize={documentPageSize}
          documentTotalPages={documentTotalPages}
          setDocumentPage={setDocumentPage}
          setDocumentPageSize={setDocumentPageSize}
        />
      ) : null}

      {tab === 'config' ? (
        <section className="panel">
          <PanelHeader
            title="动态配置"
            subtitle={`共 ${configs.total} 个配置，第 ${configPage} / ${configTotalPages} 页`}
            pageSize={configPageSize}
            setPageSize={(value) => {
              setConfigPageSize(value);
              setConfigPage(1);
            }}
          />
          <div className="table-scroll">
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
                {configs.items.map((config) => {
                  const editingSecret = Boolean(secretEditKeys[config.config_key]);
                  return (
                    <tr key={config.config_key}>
                      <td>
                        <strong>{config.config_key}</strong>
                        {config.is_secret ? <span className="tag">secret</span> : null}
                        <small>{config.domain || '-'}</small>
                      </td>
                      <td>
                        {config.is_secret && !editingSecret ? (
                          <button
                            className="button button--ghost"
                            onClick={() => setSecretEditKeys((current) => ({ ...current, [config.config_key]: true }))}
                          >
                            更新密钥
                          </button>
                        ) : config.value_type === 'text' || config.value_type === 'json' || config.config_key.startsWith('agent.prompt.') ? (
                          <textarea
                            className="table-input table-input--textarea"
                            value={configDrafts[config.config_key] ?? ''}
                            onChange={(event) => setConfigDrafts((current) => ({ ...current, [config.config_key]: event.target.value }))}
                          />
                        ) : (
                          <input
                            className="table-input"
                            type={config.is_secret ? 'password' : 'text'}
                            value={configDrafts[config.config_key] ?? ''}
                            onChange={(event) => setConfigDrafts((current) => ({ ...current, [config.config_key]: event.target.value }))}
                          />
                        )}
                      </td>
                      <td>
                        <span>{config.description}</span>
                        <small>更新时间 {formatDateTime(config.updated_at)}</small>
                      </td>
                      <td>
                        <button
                          className="button button--ghost"
                          disabled={savingKey === config.config_key || (config.is_secret && !editingSecret)}
                          onClick={() => saveConfig(config)}
                        >
                          {savingKey === config.config_key ? '保存中' : config.is_secret && !editingSecret ? '保持原值' : '保存'}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <PanelMessage loading={configs.loading} error={configs.error} empty={!configs.items.length ? '暂无配置' : ''} />
          <PaginationBar
            loading={configs.loading}
            page={configPage}
            totalPages={configTotalPages}
            visibleCount={configs.items.length}
            total={configs.total}
            onPrev={() => setConfigPage((current) => Math.max(1, current - 1))}
            onNext={() => setConfigPage((current) => Math.min(configTotalPages, current + 1))}
          />
        </section>
      ) : null}
    </section>
  );
}

function KnowledgeSection({
  token,
  documents,
  documentPage,
  documentPageSize,
  documentTotalPages,
  setDocumentPage,
  setDocumentPageSize
}: {
  token: string;
  documents: ReturnType<typeof useAsyncList<DocumentItem>>;
  documentPage: number;
  documentPageSize: number;
  documentTotalPages: number;
  setDocumentPage: (updater: number | ((current: number) => number)) => void;
  setDocumentPageSize: (value: number) => void;
}) {
  const [form, setForm] = useState({
    merchantId: 'm_001',
    title: '',
    sourceType: 'html',
    sourceUrl: '',
    content: '',
    metadata: '{\n  "platform": "external_web",\n  "category": ""\n}',
    forceReindex: false
  });
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<UnstructuredIngestionResult>();
  const notice = useNotice();
  const sourcePreview = useMemo(() => {
    const content = form.content.trim();
    if (!content) return '';
    return content.length > 300 ? `${content.slice(0, 300)}...` : content;
  }, [form.content]);

  async function submit() {
    const merchantId = form.merchantId.trim();
    const title = form.title.trim();
    const sourceUrl = form.sourceUrl.trim();
    const content = form.content.trim();
    if (!merchantId) {
      notice.showError('请填写商家 ID');
      return;
    }
    if (!title) {
      notice.showError('请填写标题');
      return;
    }
    if (!sourceUrl && !content) {
      notice.showError('请填写 URL 或原始内容');
      return;
    }
    let metadata: Record<string, unknown> | undefined;
    if (form.metadata.trim()) {
      try {
        metadata = JSON.parse(form.metadata);
      } catch {
        notice.showError('Metadata JSON 格式不合法');
        return;
      }
    }
    setSubmitting(true);
    setResult(undefined);
    notice.clear();
    try {
      const sourceType = form.sourceType as 'html' | 'json' | 'text';
      const data = await createAdminUnstructuredIngestion(token, {
        merchant_id: merchantId,
        title,
        source_type: sourceType,
        source_url: sourceUrl || undefined,
        content: sourceType === 'text' ? content || undefined : undefined,
        html: sourceType === 'html' ? content || undefined : undefined,
        json_text: sourceType === 'json' ? content || undefined : undefined,
        metadata,
        force_reindex: form.forceReindex
      });
      setResult(data);
      notice.showNotice(data.duplicate ? '内容已存在，已返回已有文档' : '入库成功');
      setDocumentPage(1);
      await documents.reload();
    } catch (error) {
      notice.showError(error, '入库失败');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <section className="panel ingestion-panel">
        <div className="panel-title-row">
          <div>
            <h2>非结构化资料入库</h2>
            <p>抓取公开网页或粘贴文本，清洗后写入知识库并切分 chunk。</p>
          </div>
          <div className="panel-actions-inline">
            <button className="button button--ghost" disabled={submitting} onClick={() => setResult(undefined)}>
              清空结果
            </button>
            <button className="button" disabled={submitting} onClick={submit}>
              {submitting ? '入库中' : '开始入库'}
            </button>
          </div>
        </div>
        <div className="ingestion-form">
          <label>
            <span>商家 ID</span>
            <input className="table-input" value={form.merchantId} onChange={(event) => setForm((current) => ({ ...current, merchantId: event.target.value }))} />
          </label>
          <label>
            <span>来源类型</span>
            <select className="table-input" value={form.sourceType} onChange={(event) => setForm((current) => ({ ...current, sourceType: event.target.value }))}>
              <option value="html">HTML / 网页</option>
              <option value="text">纯文本 / Markdown</option>
              <option value="json">JSON</option>
            </select>
          </label>
          <label className="ingestion-form__wide">
            <span>标题</span>
            <input className="table-input" value={form.title} onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))} />
          </label>
          <label className="ingestion-form__wide">
            <span>URL</span>
            <input className="table-input" value={form.sourceUrl} onChange={(event) => setForm((current) => ({ ...current, sourceUrl: event.target.value }))} />
          </label>
          <label className="ingestion-form__wide">
            <span>原始内容</span>
            <textarea className="table-input table-input--textarea ingestion-form__textarea" value={form.content} onChange={(event) => setForm((current) => ({ ...current, content: event.target.value }))} />
          </label>
          <label className="ingestion-form__wide">
            <span>Metadata JSON</span>
            <textarea className="table-input table-input--textarea ingestion-form__metadata" value={form.metadata} onChange={(event) => setForm((current) => ({ ...current, metadata: event.target.value }))} />
          </label>
          <label className="ingestion-form__checkbox">
            <input type="checkbox" checked={form.forceReindex} onChange={(event) => setForm((current) => ({ ...current, forceReindex: event.target.checked }))} />
            <span>强制重建，不按内容 hash 去重</span>
          </label>
        </div>
        {notice.notice ? <p className="notice">{notice.notice}</p> : null}
        {notice.error ? <p className="form-error">{notice.error}</p> : null}
        {sourcePreview ? (
          <details className="ingestion-preview">
            <summary>原始内容预览</summary>
            <pre>{sourcePreview}</pre>
          </details>
        ) : null}
        {result ? (
          <div className="ingestion-result">
            <span>文档 {result.document.document_id}</span>
            <span>Chunk {result.document.chunk_count}</span>
            <span>文本 {result.text_runes} 字</span>
            <span>{result.duplicate ? '重复内容' : '新文档'}</span>
            {result.document.content_hash ? <span>Hash {result.document.content_hash.slice(0, 12)}</span> : null}
          </div>
        ) : null}
      </section>

      <section className="panel">
        <PanelHeader
          title="文档"
          subtitle={`共 ${documents.total} 个文档，第 ${documentPage} / ${documentTotalPages} 页`}
          pageSize={documentPageSize}
          setPageSize={(value) => {
            setDocumentPageSize(value);
            setDocumentPage(1);
          }}
        />
        <div className="table-scroll">
          <table className="data-table">
            <thead>
              <tr>
                <th>标题</th>
                <th>类型</th>
                <th>状态</th>
                <th>Chunk</th>
                <th>来源</th>
                <th>Hash</th>
              </tr>
            </thead>
            <tbody>
              {documents.items.map((document) => (
                <tr key={document.documentId}>
                  <td>{document.title}</td>
                  <td>{document.docType}</td>
                  <td>{document.status}</td>
                  <td>{document.chunkCount}</td>
                  <td>{document.sourceUrl ? shortURL(document.sourceUrl) : '-'}</td>
                  <td>{document.contentHash ? document.contentHash.slice(0, 12) : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <PanelMessage loading={documents.loading} error={documents.error} empty={!documents.items.length ? '暂无文档' : ''} />
        <PaginationBar
          loading={documents.loading}
          page={documentPage}
          totalPages={documentTotalPages}
          visibleCount={documents.items.length}
          total={documents.total}
          onPrev={() => setDocumentPage((current) => Math.max(1, current - 1))}
          onNext={() => setDocumentPage((current) => Math.min(documentTotalPages, current + 1))}
        />
      </section>
    </>
  );
}

function PanelHeader({
  title,
  subtitle,
  pageSize,
  setPageSize
}: {
  title: string;
  subtitle: string;
  pageSize: number;
  setPageSize: (value: number) => void;
}) {
  return (
    <div className="panel-title-row">
      <div>
        <h2>{title}</h2>
        <p>{subtitle}</p>
      </div>
      <PageSizeSelect value={pageSize} onChange={setPageSize} />
    </div>
  );
}

function SummaryCard({ title, value, loading }: { title: string; value: number; loading: boolean }) {
  return (
    <article className="risk-metric">
      <span>{title}</span>
      <strong>{loading ? '-' : value}</strong>
      <small>当前管理员视图</small>
    </article>
  );
}

function reloadActiveTab(
  tab: PlatformTab,
  loaders: Record<'accounts' | 'products' | 'orders' | 'documents' | 'configs', { reload: () => Promise<void> }>
) {
  if (tab === 'overview') {
    void Promise.all([loaders.accounts.reload(), loaders.products.reload(), loaders.orders.reload(), loaders.documents.reload()]);
  }
  if (tab === 'catalog') void loaders.products.reload();
  if (tab === 'orders') void loaders.orders.reload();
  if (tab === 'knowledge') void loaders.documents.reload();
  if (tab === 'config') void loaders.configs.reload();
}

function validateConfigValue(config: AppConfig, value: string, editingSecret: boolean) {
  if (config.is_secret && !editingSecret) return '';
  if (config.is_secret && !value.trim()) return '密钥配置不能为空；如不修改请保持原值';
  if (config.value_type === 'json') {
    try {
      JSON.parse(value);
    } catch {
      return `${config.config_key} 不是合法 JSON`;
    }
  }
  if (config.value_type === 'int' && !Number.isInteger(Number(value))) {
    return `${config.config_key} 需要填写整数`;
  }
  if (config.value_type === 'bool' && !['true', 'false'].includes(value.trim().toLowerCase())) {
    return `${config.config_key} 需要填写 true 或 false`;
  }
  return '';
}

