import { useEffect, useState } from 'react';
import {
  DocumentItem,
  EvalRun,
  AppConfig,
  listAccounts,
  listAppConfigs,
  listAdminOrders,
  listAdminProducts,
  listDocuments,
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
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [evalRuns, setEvalRuns] = useState<EvalRun[]>([]);
  const [products, setProducts] = useState<ProductCard[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [configs, setConfigs] = useState<AppConfig[]>([]);
  const [configDrafts, setConfigDrafts] = useState<Record<string, string>>({});

  useEffect(() => {
    refresh();
    listEvalRuns().then(setEvalRuns);
  }, [token]);

  async function refresh() {
    const [nextAccounts, nextDocuments, nextProducts, nextOrders, nextConfigs] = await Promise.all([
      listAccounts(token),
      listDocuments(token),
      listAdminProducts(token),
      listAdminOrders(token),
      listAppConfigs(token)
    ]);
    setAccounts(nextAccounts);
    setDocuments(nextDocuments);
    setProducts(nextProducts);
    setOrders(nextOrders);
    setConfigs(nextConfigs);
    setConfigDrafts(Object.fromEntries(nextConfigs.map((item) => [item.config_key, item.config_value])));
  }

  async function toggleAccount(account: Account) {
    await updateAccountStatus(token, account.account_id, account.status === 'active' ? 'inactive' : 'active');
    await refresh();
  }

  async function disableProduct(product: ProductCard) {
    await updateAdminProductStatus(token, product.productId, 'inactive');
    await refresh();
  }

  async function saveConfig(config: AppConfig) {
    await updateAppConfig(token, config.config_key, {
      value: configDrafts[config.config_key] ?? '',
      value_type: config.value_type,
      description: config.description,
      is_secret: config.is_secret
    });
    await refresh();
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
          <h2>账号</h2>
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
        </section>
        <section className="panel">
          <h2>商品</h2>
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
        </section>
        <section className="panel">
          <h2>订单</h2>
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
        </section>
        <section className="panel">
          <h2>文档</h2>
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
        </section>
        <section className="panel">
          <h2>动态配置</h2>
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
