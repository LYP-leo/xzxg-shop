import type { ProductCard as ProductCardType } from '../../types/product';

type Props = {
  product: ProductCardType;
  onOpen?: (productId: string) => void;
  onAddToCart?: (product: ProductCardType) => void;
};

export function ProductCard({ product, onOpen, onAddToCart }: Props) {
  return (
    <article className="product-card">
      <button className="product-card__image" onClick={() => onOpen?.(product.productId)} aria-label={`查看 ${product.name}`}>
        <img src={product.imageUrl} alt={product.name} />
      </button>
      <div className="product-card__body">
        <div className="product-card__meta">
          <span>{product.merchantName}</span>
          <span>{product.stockStatus === 'in_stock' ? '有货' : '缺货'}</span>
        </div>
        <button className="product-card__title" onClick={() => onOpen?.(product.productId)}>
          {product.name}
        </button>
        <div className="product-card__price">¥{product.price}</div>
        <div className="tag-row">
          {product.tags.slice(0, 4).map((tag) => (
            <span className="tag" key={tag}>
              {tag}
            </span>
          ))}
        </div>
        {product.recommendReason ? <p className="product-card__reason">{product.recommendReason}</p> : null}
        {product.riskNotes?.length ? <p className="product-card__risk">风险：{product.riskNotes.join('、')}</p> : null}
        <div className="product-card__actions">
          <button className="button button--ghost" onClick={() => onOpen?.(product.productId)}>
            详情
          </button>
          {onAddToCart ? (
            <button className="button" disabled={product.stockStatus !== 'in_stock'} onClick={() => onAddToCart(product)}>
              加购
            </button>
          ) : null}
        </div>
      </div>
    </article>
  );
}
