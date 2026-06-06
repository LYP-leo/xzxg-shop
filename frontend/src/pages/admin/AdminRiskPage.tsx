import { useState } from 'react';
import {
  AppConfig,
  MerchantStatus,
  ProductStatus,
  listAccountsPage,
  listAdminMerchants,
  listAdminProducts,
  listAppConfigsPage,
  updateAccountStatus,
  updateAdminMerchantStatus,
  updateAdminProductStatus,
  updateAppConfig
} from '../../api/admin';
import type { Account } from '../../types/auth';
import type { Merchant, ProductCard } from '../../types/product';
import {
  PageSizeSelect,
  PaginationBar,
  PanelMessage,
  RiskMetric,
  RiskStatusSelect,
  StatusBadge,
  productStatus,
  totalPages,
  useAsyncList,
  useNotice
} from './AdminCommon';

export function AdminRiskPage({ token }: { token: string }) {
  const [accountPage, setAccountPage] = useState(1);
  const [accountPageSize, setAccountPageSize] = useState(10);
  const [productPage, setProductPage] = useState(1);
  const [productPageSize, setProductPageSize] = useState(10);
  const [merchantPage, setMerchantPage] = useState(1);
  const [merchantPageSize, setMerchantPageSize] = useState(10);
  const [configDrafts, setConfigDrafts] = useState<Record<string, string>>({});
  const [mutatingId, setMutatingId] = useState('');
  const notice = useNotice();
  const accounts = useAsyncList<Account>(() => listAccountsPage(token, accountPage, accountPageSize), [token, accountPage, accountPageSize]);
  const products = useAsyncList<ProductCard>(() => listAdminProducts(token, productPage, productPageSize), [token, productPage, productPageSize]);
  const merchants = useAsyncList<Merchant>(() => listAdminMerchants(token, merchantPage, merchantPageSize), [token, merchantPage, merchantPageSize]);
  const configs = useAsyncList<AppConfig>(
    async () => {
      const data = await listAppConfigsPage(token, 1, 100);
      setConfigDrafts((current) => ({
        ...current,
        ...Object.fromEntries(data.items.map((item) => [item.config_key, current[item.config_key] ?? item.config_value]))
      }));
      return { ...data, items: data.items.filter((config) => config.config_key.startsWith('risk.')) };
    },
    [token]
  );
  const accountPages = totalPages(accounts.total, accountPageSize);
  const productPages = totalPages(products.total, productPageSize);
  const merchantPages = totalPages(merchants.total, merchantPageSize);

  async function setAccountRiskStatus(account: Account, status: 'active' | 'inactive' | 'risk') {
    await mutate(account.account_id, async () => {
      await updateAccountStatus(token, account.account_id, status);
      await accounts.reload();
    });
  }

  async function setProductRiskStatus(product: ProductCard, status: ProductStatus) {
    await mutate(product.productId, async () => {
      await updateAdminProductStatus(token, product.productId, status);
      await products.reload();
    });
  }

  async function setMerchantRiskStatus(merchant: Merchant, status: MerchantStatus) {
    await mutate(merchant.merchantId, async () => {
      await updateAdminMerchantStatus(token, merchant.merchantId, status);
      await merchants.reload();
    });
  }

  async function saveConfig(config: AppConfig) {
    const value = configDrafts[config.config_key] ?? '';
    if (config.value_type === 'json') {
      try {
        JSON.parse(value);
      } catch {
        notice.showError(`${config.config_key} 不是合法 JSON`);
        return;
      }
    }
    await mutate(config.config_key, async () => {
      await updateAppConfig(token, config.config_key, {
        value,
        value_type: config.value_type,
        description: config.description,
        is_secret: config.is_secret
      });
      await configs.reload();
    });
  }

  async function mutate(id: string, handler: () => Promise<void>) {
    setMutatingId(id);
    notice.clear();
    try {
      await handler();
      notice.showNotice('风控配置已更新');
    } catch (error) {
      notice.showError(error);
    } finally {
      setMutatingId('');
    }
  }

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>风控管理</h1>
          <p>管理输入内容风控、风险用户、风险商家和风险商品。风险对象不会进入 Agent 召回链路。</p>
        </div>
        <button className="button button--ghost" onClick={() => void Promise.all([accounts.reload(), products.reload(), merchants.reload(), configs.reload()])}>
          刷新
        </button>
      </header>
      {notice.notice ? <p className="notice">{notice.notice}</p> : null}
      {notice.error ? <p className="form-error">{notice.error}</p> : null}
      <div className="risk-summary-grid">
        <RiskMetric title="风险账号" value={accounts.items.filter((account) => account.status === 'risk').length} hint={`当前页 / 共 ${accounts.total} 个账号`} />
        <RiskMetric title="风险商品" value={products.items.filter((product) => productStatus(product) === 'risk').length} hint={`当前页 / 共 ${products.total} 个商品`} />
        <RiskMetric title="风险商家" value={merchants.items.filter((merchant) => merchant.status === 'risk').length} hint={`当前页 / 共 ${merchants.total} 个商家`} />
        <RiskMetric title="风控配置" value={configs.items.length} hint="来自 Nacos 动态配置" />
      </div>
      <div className="admin-grid">
        <section className="panel risk-panel">
          <div className="panel-title-row">
            <div>
              <h2>输入内容风控</h2>
              <p>维护 risk.* 配置，保存后后端按动态配置刷新间隔生效。</p>
            </div>
          </div>
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
                {configs.items.map((config) => (
                  <tr key={config.config_key}>
                    <td>
                      <strong>{config.config_key}</strong>
                    </td>
                    <td>
                      {config.value_type === 'json' || config.value_type === 'text' ? (
                        <textarea
                          className="table-input table-input--textarea"
                          value={configDrafts[config.config_key] ?? ''}
                          onChange={(event) => setConfigDrafts((current) => ({ ...current, [config.config_key]: event.target.value }))}
                        />
                      ) : (
                        <input
                          className="table-input"
                          value={configDrafts[config.config_key] ?? ''}
                          onChange={(event) => setConfigDrafts((current) => ({ ...current, [config.config_key]: event.target.value }))}
                        />
                      )}
                    </td>
                    <td>{config.description}</td>
                    <td>
                      <button className="button button--ghost" disabled={mutatingId === config.config_key} onClick={() => saveConfig(config)}>
                        {mutatingId === config.config_key ? '保存中' : '保存'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <PanelMessage loading={configs.loading} error={configs.error} empty={!configs.items.length ? '暂无 risk.* 配置' : ''} />
        </section>
        <RiskAccountTable
          accounts={accounts.items}
          total={accounts.total}
          loading={accounts.loading}
          error={accounts.error}
          page={accountPage}
          pages={accountPages}
          pageSize={accountPageSize}
          mutatingId={mutatingId}
          setPage={setAccountPage}
          setPageSize={setAccountPageSize}
          onChange={setAccountRiskStatus}
        />
        <RiskMerchantTable
          merchants={merchants.items}
          total={merchants.total}
          loading={merchants.loading}
          error={merchants.error}
          page={merchantPage}
          pages={merchantPages}
          pageSize={merchantPageSize}
          mutatingId={mutatingId}
          setPage={setMerchantPage}
          setPageSize={setMerchantPageSize}
          onChange={setMerchantRiskStatus}
        />
        <RiskProductTable
          products={products.items}
          total={products.total}
          loading={products.loading}
          error={products.error}
          page={productPage}
          pages={productPages}
          pageSize={productPageSize}
          mutatingId={mutatingId}
          setPage={setProductPage}
          setPageSize={setProductPageSize}
          onChange={setProductRiskStatus}
        />
      </div>
    </section>
  );
}

function RiskAccountTable(props: {
  accounts: Account[];
  total: number;
  loading: boolean;
  error: string;
  page: number;
  pages: number;
  pageSize: number;
  mutatingId: string;
  setPage: (updater: number | ((current: number) => number)) => void;
  setPageSize: (value: number) => void;
  onChange: (account: Account, status: 'active' | 'inactive' | 'risk') => void;
}) {
  return (
    <section className="panel">
      <RiskTableHeader title="风险用户" subtitle={`共 ${props.total} 个账号，第 ${props.page} / ${props.pages} 页`} pageSize={props.pageSize} setPageSize={props.setPageSize} setPage={props.setPage} />
      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              <th>账号</th>
              <th>角色</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {props.accounts.map((account) => (
              <tr key={account.account_id}>
                <td>{account.username}</td>
                <td>{account.role}</td>
                <td>
                  <StatusBadge status={account.status || 'active'} />
                </td>
                <td>
                  <RiskStatusSelect
                    disabled={props.mutatingId === account.account_id}
                    value={(account.status || 'active') as 'active' | 'inactive' | 'risk'}
                    options={['active', 'inactive', 'risk']}
                    onChange={(status) => props.onChange(account, status as 'active' | 'inactive' | 'risk')}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <PanelMessage loading={props.loading} error={props.error} empty={!props.accounts.length ? '暂无账号' : ''} />
      <PaginationBar loading={props.loading} page={props.page} totalPages={props.pages} visibleCount={props.accounts.length} total={props.total} onPrev={() => props.setPage((current) => Math.max(1, current - 1))} onNext={() => props.setPage((current) => Math.min(props.pages, current + 1))} />
    </section>
  );
}

function RiskMerchantTable(props: {
  merchants: Merchant[];
  total: number;
  loading: boolean;
  error: string;
  page: number;
  pages: number;
  pageSize: number;
  mutatingId: string;
  setPage: (updater: number | ((current: number) => number)) => void;
  setPageSize: (value: number) => void;
  onChange: (merchant: Merchant, status: MerchantStatus) => void;
}) {
  return (
    <section className="panel">
      <RiskTableHeader title="风险商家" subtitle={`共 ${props.total} 个商家，第 ${props.page} / ${props.pages} 页`} pageSize={props.pageSize} setPageSize={props.setPageSize} setPage={props.setPage} />
      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              <th>商家</th>
              <th>电话</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {props.merchants.map((merchant) => (
              <tr key={merchant.merchantId}>
                <td>{merchant.name}</td>
                <td>{merchant.servicePhone || '-'}</td>
                <td>
                  <StatusBadge status={merchant.status} />
                </td>
                <td>
                  <RiskStatusSelect
                    disabled={props.mutatingId === merchant.merchantId}
                    value={(merchant.status || 'active') as MerchantStatus}
                    options={['active', 'inactive', 'risk']}
                    onChange={(status) => props.onChange(merchant, status as MerchantStatus)}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <PanelMessage loading={props.loading} error={props.error} empty={!props.merchants.length ? '暂无商家' : ''} />
      <PaginationBar loading={props.loading} page={props.page} totalPages={props.pages} visibleCount={props.merchants.length} total={props.total} onPrev={() => props.setPage((current) => Math.max(1, current - 1))} onNext={() => props.setPage((current) => Math.min(props.pages, current + 1))} />
    </section>
  );
}

function RiskProductTable(props: {
  products: ProductCard[];
  total: number;
  loading: boolean;
  error: string;
  page: number;
  pages: number;
  pageSize: number;
  mutatingId: string;
  setPage: (updater: number | ((current: number) => number)) => void;
  setPageSize: (value: number) => void;
  onChange: (product: ProductCard, status: ProductStatus) => void;
}) {
  return (
    <section className="panel">
      <RiskTableHeader title="风险商品" subtitle={`共 ${props.total} 个商品，第 ${props.page} / ${props.pages} 页`} pageSize={props.pageSize} setPageSize={props.setPageSize} setPage={props.setPage} />
      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              <th>商品</th>
              <th>商家</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {props.products.map((product) => (
              <tr key={product.productId}>
                <td>{product.name}</td>
                <td>{product.merchantName}</td>
                <td>
                  <StatusBadge status={productStatus(product)} />
                </td>
                <td>
                  <RiskStatusSelect
                    disabled={props.mutatingId === product.productId}
                    value={productStatus(product) as ProductStatus}
                    options={['active', 'inactive', 'risk', 'deleted']}
                    onChange={(status) => props.onChange(product, status as ProductStatus)}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <PanelMessage loading={props.loading} error={props.error} empty={!props.products.length ? '暂无商品' : ''} />
      <PaginationBar loading={props.loading} page={props.page} totalPages={props.pages} visibleCount={props.products.length} total={props.total} onPrev={() => props.setPage((current) => Math.max(1, current - 1))} onNext={() => props.setPage((current) => Math.min(props.pages, current + 1))} />
    </section>
  );
}

function RiskTableHeader({
  title,
  subtitle,
  pageSize,
  setPageSize,
  setPage
}: {
  title: string;
  subtitle: string;
  pageSize: number;
  setPageSize: (value: number) => void;
  setPage: (updater: number | ((current: number) => number)) => void;
}) {
  return (
    <div className="panel-title-row">
      <div>
        <h2>{title}</h2>
        <p>{subtitle}</p>
      </div>
      <PageSizeSelect
        value={pageSize}
        onChange={(value) => {
          setPageSize(value);
          setPage(1);
        }}
      />
    </div>
  );
}

