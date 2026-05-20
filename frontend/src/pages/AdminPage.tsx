import { useEffect, useState } from 'react';
import {
  DocumentItem,
  EvalRun,
  listAccounts,
  listAdminOrders,
  listAdminProducts,
  listDocuments,
  listEvalRuns,
  updateAccountStatus,
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

  useEffect(() => {
    refresh();
    listEvalRuns().then(setEvalRuns);
  }, [token]);

  async function refresh() {
    const [nextAccounts, nextDocuments, nextProducts, nextOrders] = await Promise.all([
      listAccounts(token),
      listDocuments(token),
      listAdminProducts(token),
      listAdminOrders(token)
    ]);
    setAccounts(nextAccounts);
    setDocuments(nextDocuments);
    setProducts(nextProducts);
    setOrders(nextOrders);
  }

  async function toggleAccount(account: Account) {
    await updateAccountStatus(token, account.account_id, account.status === 'active' ? 'inactive' : 'active');
    await refresh();
  }

  async function disableProduct(product: ProductCard) {
    await updateAdminProductStatus(token, product.productId, 'inactive');
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
