import { categories, merchants, products, skus, getProductCards } from '../mock/data';
import type { Category, Merchant, ProductCard, ProductDetail, ProductSku } from '../types/product';
import { requestJSON } from './http';

export async function listCategories(): Promise<Category[]> {
  try {
    const data = await requestJSON<{ items: Category[] }>('/categories/tree');
    return data.items;
  } catch {
    return categories;
  }
}

export async function listMerchants(): Promise<Merchant[]> {
  try {
    const data = await requestJSON<{ items: Merchant[] }>('/merchants');
    return data.items;
  } catch {
    return merchants;
  }
}

export async function listProducts(params: { keyword?: string; categoryId?: string } = {}): Promise<ProductCard[]> {
  try {
    const query = new URLSearchParams();
    if (params.keyword) query.set('keyword', params.keyword);
    if (params.categoryId) query.set('category_id', params.categoryId);
    const data = await requestJSON<{ items: ProductCard[] }>(`/products?${query.toString()}`);
    return data.items;
  } catch {
    const keyword = params.keyword?.trim().toLowerCase();
    return getProductCards().filter((product) => {
      const matchesKeyword = !keyword || `${product.name}${product.brand}${product.tags.join('')}`.toLowerCase().includes(keyword);
      const matchesCategory = !params.categoryId || product.categoryId === params.categoryId;
      return matchesKeyword && matchesCategory;
    });
  }
}

export async function getProduct(productId: string): Promise<ProductDetail | undefined> {
  try {
    return await requestJSON<ProductDetail>(`/products/${productId}`);
  } catch {
    return products.find((product) => product.productId === productId);
  }
}

export async function listProductSkus(productId: string): Promise<ProductSku[]> {
  try {
    const data = await requestJSON<{ items: ProductSku[] }>(`/products/${productId}/skus`);
    return data.items;
  } catch {
    return skus.filter((sku) => sku.productId === productId);
  }
}
