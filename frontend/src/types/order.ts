export type OrderItem = {
  order_item_id: string;
  product_id: string;
  sku_id?: string;
  name: string;
  image_url: string;
  price: string;
  quantity: number;
  merchant_id: string;
  merchant_name: string;
};

export type Order = {
  order_id: string;
  account_id: string;
  merchant_id: string;
  merchant_name: string;
  status: 'pending_ship' | 'shipped' | 'completed' | 'canceled';
  total_amount: string;
  items: OrderItem[];
  created_at: string;
};
