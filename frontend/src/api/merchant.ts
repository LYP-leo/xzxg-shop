import type { ProductDetail } from '../types/product';
import { requestJSON } from './http';

export type MerchantProductInput = {
  name: string;
  brand: string;
  category_id: string;
  image_url: string;
  price: string;
  market_price: string;
  stock_quantity: number;
  stock_status: string;
  tags: string[];
  selling_points: string[];
  recommend_reason: string;
  risk_notes: string[];
  description: string;
};

export type KnowledgeDocument = {
  document_id: string;
  merchant_id: string;
  title: string;
  doc_type: string;
  status: string;
  chunk_count: number;
  created_at: string;
};

export async function createMerchantProduct(token: string, input: MerchantProductInput): Promise<ProductDetail> {
  return requestJSON<ProductDetail>('/merchant/products', {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(input)
  });
}

export async function updateMerchantProduct(token: string, productId: string, input: MerchantProductInput): Promise<ProductDetail> {
  return requestJSON<ProductDetail>(`/merchant/products/${productId}`, {
    method: 'PATCH',
    headers: authHeaders(token),
    body: JSON.stringify(input)
  });
}

export async function listMerchantDocuments(token: string): Promise<KnowledgeDocument[]> {
  const data = await requestJSON<{ items: KnowledgeDocument[] }>('/merchant/documents', {
    headers: authHeaders(token)
  });
  return data.items;
}

export async function uploadMerchantDocument(
  token: string,
  input: { title: string; doc_type: string; content: string }
): Promise<KnowledgeDocument> {
  return requestJSON<KnowledgeDocument>('/merchant/documents', {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(input)
  });
}

function authHeaders(token: string) {
  return {
    Authorization: `Bearer ${token}`
  };
}
