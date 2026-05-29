export type CartItem = {
  cartItemId: string;
  productId: string;
  skuId?: string;
  name: string;
  imageUrl: string;
  price: string;
  quantity: number;
  selected: boolean;
  stockStatus: 'in_stock' | 'out_of_stock' | 'pre_sale';
  merchantId: string;
  merchantName: string;
};

export type CartSummary = {
  selectedCount: number;
  totalAmount: string;
  discountAmount: string;
  payAmount: string;
};

export type Cart = {
  items: CartItem[];
  summary: CartSummary;
};
