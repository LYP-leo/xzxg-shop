import { useEffect, useState } from 'react';
import { getProduct, listProductSkus } from '../api/product';
import type { ProductDetail, ProductSku } from '../types/product';

type Props = {
  productId: string;
  onBack: () => void;
};

export function ProductDetailPage({ productId, onBack }: Props) {
  const [product, setProduct] = useState<ProductDetail>();
  const [skus, setSkus] = useState<ProductSku[]>([]);
  const [selectedSkuId, setSelectedSkuId] = useState<string>();

  useEffect(() => {
    getProduct(productId).then(setProduct);
    listProductSkus(productId).then((items) => {
      setSkus(items);
      setSelectedSkuId(items[0]?.skuId);
    });
  }, [productId]);

  if (!product) {
    return <div className="empty-state">正在加载商品...</div>;
  }

  const currentProduct = product;

  return (
    <section>
      <button className="button button--ghost" onClick={onBack}>
        返回
      </button>
      <div className="detail-layout">
        <div className="detail-image">
          <img src={product.imageUrl} alt={product.name} />
        </div>
        <div className="detail-panel">
          <div className="product-card__meta">
            <span>{currentProduct.merchantName}</span>
            <span>{currentProduct.stockStatus === 'in_stock' ? '有货' : '缺货'}</span>
          </div>
          <h1>{currentProduct.name}</h1>
          <div className="detail-price">¥{currentProduct.price}</div>
          <p>{currentProduct.description}</p>
          <div className="tag-row">
            {currentProduct.tags.map((tag) => (
              <span className="tag" key={tag}>
                {tag}
              </span>
            ))}
          </div>
          <label className="field">
            SKU
            <select value={selectedSkuId} onChange={(event) => setSelectedSkuId(event.target.value)}>
              {skus.map((sku) => (
                <option value={sku.skuId} key={sku.skuId}>
                  {sku.skuName}
                </option>
              ))}
            </select>
          </label>
          <div className="detail-actions">
            <span className="pill">商品 ID：{currentProduct.productId}</span>
            <span className="pill">SKU：{selectedSkuId || '-'}</span>
          </div>
        </div>
      </div>
      <section className="panel">
        <h2>关键属性</h2>
        <div className="attribute-grid">
          {currentProduct.attributes.map((attribute) => (
            <div key={attribute.key}>
              <span>{attribute.key}</span>
              <strong>
                {attribute.value}
                {attribute.unit ?? ''}
              </strong>
            </div>
          ))}
        </div>
      </section>
    </section>
  );
}
