import { useEffect, useState } from 'react';
import { addToCart } from '../api/cart';
import { listCategories, listProducts } from '../api/product';
import { ProductCard } from '../components/product/ProductCard';
import type { Category, ProductCard as ProductCardType } from '../types/product';

type Props = {
  onOpenProduct: (productId: string) => void;
  onCartChange: () => void;
};

export function ProductListPage({ onOpenProduct, onCartChange }: Props) {
  const [keyword, setKeyword] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [categories, setCategories] = useState<Category[]>([]);
  const [products, setProducts] = useState<ProductCardType[]>([]);

  useEffect(() => {
    listCategories().then(setCategories);
  }, []);

  useEffect(() => {
    listProducts({ keyword, categoryId }).then(setProducts);
  }, [keyword, categoryId]);

  async function addProduct(product: ProductCardType) {
    await addToCart({ productId: product.productId, skuId: product.skuId, quantity: 1 });
    onCartChange();
  }

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>商品</h1>
          <p>最小电商域商品数据，供 Agent 推荐和购物车使用。</p>
        </div>
      </header>
      <div className="toolbar">
        <input value={keyword} placeholder="搜索商品、品牌或标签" onChange={(event) => setKeyword(event.target.value)} />
        <select value={categoryId} onChange={(event) => setCategoryId(event.target.value)}>
          <option value="">全部类目</option>
          {categories.map((category) => (
            <option value={category.categoryId} key={category.categoryId}>
              {category.name}
            </option>
          ))}
        </select>
      </div>
      <div className="product-grid">
        {products.map((product) => (
          <ProductCard product={product} key={product.productId} onOpen={onOpenProduct} onAddToCart={addProduct} />
        ))}
      </div>
    </section>
  );
}
