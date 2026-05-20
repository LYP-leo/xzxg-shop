import { useEffect, useState } from 'react';
import { addToCart } from '../api/cart';
import { getProduct, listProductSkus } from '../api/product';
import type { ProductDetail, ProductSku } from '../types/product';

type Props = {
  productId: string;
  onBack: () => void;
  onAskAgent: (question: string) => void;
  onCartChange: () => void;
  canAddToCart?: boolean;
  canAskAgent?: boolean;
};

export function ProductDetailPage({ productId, onBack, onAskAgent, onCartChange, canAddToCart = true, canAskAgent = true }: Props) {
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

  async function addProduct() {
    await addToCart({ productId: currentProduct.productId, skuId: selectedSkuId, quantity: 1 });
    onCartChange();
  }

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
            {canAddToCart ? (
              <button className="button" onClick={addProduct}>
                加入购物车
              </button>
            ) : null}
            {canAskAgent ? (
              <button className="button button--ghost" onClick={() => onAskAgent(`帮我分析 ${currentProduct.name} 是否适合我`)}>
                问问 Agent
              </button>
            ) : null}
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
