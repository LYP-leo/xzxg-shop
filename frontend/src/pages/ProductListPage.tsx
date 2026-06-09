import { useEffect, useState } from 'react';
import { listCategories, listProducts } from '../api/product';
import { ProductCard } from '../components/product/ProductCard';
import type { Category, ProductCard as ProductCardType } from '../types/product';

type Props = {
  onOpenProduct: (productId: string) => void;
};

export function ProductListPage({ onOpenProduct }: Props) {
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

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>商品</h1>
          <p>查看当前商品库数据、库存状态和风险标记，用于后台巡检。</p>
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
          <ProductCard product={product} key={product.productId} onOpen={onOpenProduct} />
        ))}
      </div>
    </section>
  );
}
