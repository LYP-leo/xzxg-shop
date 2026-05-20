import { useEffect, useState } from 'react';
import { listProducts } from '../api/product';
import type { Account } from '../types/auth';
import type { ProductCard } from '../types/product';

type MerchantPageProps = {
  account: Account;
};

export function MerchantPage({ account }: MerchantPageProps) {
  const [products, setProducts] = useState<ProductCard[]>([]);

  useEffect(() => {
    listProducts().then(setProducts);
  }, []);

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>商家工作台</h1>
          <p>{account.display_name} 正在维护商品、知识素材和导购表现。</p>
        </div>
      </header>
      <div className="metric-grid">
        <section className="metric-card">
          <span>在售商品</span>
          <strong>{products.length}</strong>
        </section>
        <section className="metric-card">
          <span>知识素材</span>
          <strong>2</strong>
        </section>
        <section className="metric-card">
          <span>待优化回答</span>
          <strong>3</strong>
        </section>
      </div>
      <section className="panel">
        <h2>商品运营</h2>
        <table className="data-table">
          <thead>
            <tr>
              <th>商品</th>
              <th>价格</th>
              <th>库存</th>
              <th>导购卖点</th>
            </tr>
          </thead>
          <tbody>
            {products.map((product) => (
              <tr key={product.productId}>
                <td>{product.name}</td>
                <td>¥{product.price}</td>
                <td>{product.stockStatus}</td>
                <td>{product.sellingPoints.join(' / ')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </section>
  );
}
