import type { Category, Merchant, ProductCard, ProductDetail, ProductSku } from '../types/product';
import { requestJSON } from './http';

export async function listCategories(): Promise<Category[]> {
  const data = await requestJSON<{ items: Category[] }>('/categories/tree');
  return data.items;
}

export async function listMerchants(): Promise<Merchant[]> {
  const data = await requestJSON<{ items: Merchant[] }>('/merchants');
  return data.items;
}

export async function listProducts(params: { keyword?: string; categoryId?: string } = {}): Promise<ProductCard[]> {
  const query = new URLSearchParams();
  if (params.keyword) query.set('keyword', params.keyword);
  if (params.categoryId) query.set('category_id', params.categoryId);
  const data = await requestJSON<{ items: ProductCard[] }>(`/products?${query.toString()}`);
  return data.items;
}

export async function getProduct(productId: string): Promise<ProductDetail | undefined> {
  return requestJSON<ProductDetail>(`/products/${productId}`);
}

export async function listProductSkus(productId: string): Promise<ProductSku[]> {
  const data = await requestJSON<{ items: ProductSku[] }>(`/products/${productId}/skus`);
  return data.items;
}
