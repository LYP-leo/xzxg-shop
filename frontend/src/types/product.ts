export type StockStatus = 'in_stock' | 'out_of_stock' | 'pre_sale';

export type Merchant = {
  merchantId: string;
  name: string;
  logoUrl: string;
  description: string;
  servicePhone?: string;
  status: 'active' | 'inactive';
};

export type Category = {
  categoryId: string;
  parentId: string;
  name: string;
  children?: Category[];
};

export type ProductAttribute = {
  key: string;
  value: string;
  unit?: string;
};

export type ProductCard = {
  productId: string;
  skuId?: string;
  merchantId: string;
  merchantName: string;
  categoryId: string;
  name: string;
  brand: string;
  imageUrl: string;
  price: string;
  marketPrice?: string;
  stockStatus: StockStatus;
  tags: string[];
  sellingPoints: string[];
  recommendReason?: string;
  riskNotes?: string[];
};

export type ProductDetail = ProductCard & {
  imageUrls: string[];
  stockQuantity: number;
  attributes: ProductAttribute[];
  suitableFor: string[];
  notSuitableFor: string[];
  description: string;
};

export type ProductSku = {
  skuId: string;
  productId: string;
  skuName: string;
  price: string;
  stockQuantity: number;
  stockStatus: StockStatus;
  specs: Record<string, string>;
};
