import type { AgentBlock, AgentSession, AgentSseEvent } from '../types/agent';
import type { Cart, CartItem } from '../types/cart';
import type { Category, Merchant, ProductCard, ProductDetail, ProductSku } from '../types/product';

export const merchants: Merchant[] = [
  {
    merchantId: 'm_001',
    name: '小猪数码旗舰店',
    logoUrl: '/placeholder-merchant.svg',
    description: '主营手机、耳机、智能设备和办公外设。',
    servicePhone: '400-000-0000',
    status: 'active'
  }
];

export const categories: Category[] = [
  {
    categoryId: 'c_phone',
    parentId: '',
    name: '手机',
    children: []
  },
  {
    categoryId: 'c_mouse',
    parentId: '',
    name: '鼠标',
    children: []
  }
];

export const products: ProductDetail[] = [
  {
    productId: 'p_001',
    skuId: 'sku_001',
    merchantId: 'm_001',
    merchantName: '小猪数码旗舰店',
    categoryId: 'c_phone',
    name: 'X Phone 12',
    brand: 'X',
    imageUrl: 'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=640&q=80',
    imageUrls: ['https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=640&q=80'],
    price: '2999.00',
    marketPrice: '3299.00',
    stockStatus: 'in_stock',
    stockQuantity: 84,
    tags: ['拍照', '预算内', '抓拍'],
    sellingPoints: ['高速对焦', '儿童抓拍模式', '256GB 存储'],
    suitableFor: ['拍娃', '日常拍照', '预算敏感'],
    notSuitableFor: ['重度游戏'],
    attributes: [
      { key: '存储', value: '256GB' },
      { key: '重量', value: '189', unit: 'g' },
      { key: '屏幕', value: '6.5 英寸 OLED' }
    ],
    description: '适合预算内拍照和日常使用的手机。'
  },
  {
    productId: 'p_002',
    skuId: 'sku_002',
    merchantId: 'm_001',
    merchantName: '小猪数码旗舰店',
    categoryId: 'c_phone',
    name: 'Y Camera Max',
    brand: 'Y',
    imageUrl: 'https://images.unsplash.com/photo-1598327105666-5b89351aff97?auto=format&fit=crop&w=640&q=80',
    imageUrls: ['https://images.unsplash.com/photo-1598327105666-5b89351aff97?auto=format&fit=crop&w=640&q=80'],
    price: '3499.00',
    marketPrice: '3899.00',
    stockStatus: 'in_stock',
    stockQuantity: 32,
    tags: ['影像旗舰', '长焦', '续航'],
    sellingPoints: ['长焦表现好', '夜景稳定', '续航更强'],
    suitableFor: ['旅行拍照', '重视续航'],
    notSuitableFor: ['严格 3000 以内预算'],
    attributes: [
      { key: '存储', value: '256GB' },
      { key: '重量', value: '204', unit: 'g' }
    ],
    description: '影像能力更强，但价格超过 3000。'
  },
  {
    productId: 'p_mouse_001',
    skuId: 'sku_mouse_001',
    merchantId: 'm_001',
    merchantName: '小猪数码旗舰店',
    categoryId: 'c_mouse',
    name: 'Quiet Mouse S',
    brand: 'Q',
    imageUrl: 'https://images.unsplash.com/photo-1527814050087-3793815479db?auto=format&fit=crop&w=640&q=80',
    imageUrls: ['https://images.unsplash.com/photo-1527814050087-3793815479db?auto=format&fit=crop&w=640&q=80'],
    price: '129.00',
    marketPrice: '159.00',
    stockStatus: 'in_stock',
    stockQuantity: 120,
    tags: ['静音', '办公', '无线'],
    sellingPoints: ['静音微动', '人体工学', '长续航'],
    suitableFor: ['办公', '宿舍', '图书馆'],
    notSuitableFor: ['高强度电竞'],
    attributes: [
      { key: '连接', value: '2.4G 无线 + 蓝牙' },
      { key: '重量', value: '88', unit: 'g' }
    ],
    description: '适合安静办公环境的无线鼠标。'
  }
];

export const skus: ProductSku[] = products.map((product) => ({
  skuId: product.skuId ?? `${product.productId}_sku`,
  productId: product.productId,
  skuName: `${product.name} 标准版`,
  price: product.price,
  stockQuantity: product.stockQuantity,
  stockStatus: product.stockStatus,
  specs: { 版本: '标准版' }
}));

let cartItems: CartItem[] = [];

export function getProductCards(): ProductCard[] {
  return products.map((product) => ({
    productId: product.productId,
    skuId: product.skuId,
    merchantId: product.merchantId,
    merchantName: product.merchantName,
    categoryId: product.categoryId,
    name: product.name,
    brand: product.brand,
    imageUrl: product.imageUrl,
    price: product.price,
    marketPrice: product.marketPrice,
    stockStatus: product.stockStatus,
    tags: product.tags,
    sellingPoints: product.sellingPoints
  }));
}

export function getCart(): Cart {
  const selected = cartItems.filter((item) => item.selected);
  const total = selected.reduce((sum, item) => sum + Number(item.price) * item.quantity, 0);
  return {
    items: cartItems,
    summary: {
      selectedCount: selected.reduce((sum, item) => sum + item.quantity, 0),
      totalAmount: total.toFixed(2),
      discountAmount: '0.00',
      payAmount: total.toFixed(2)
    }
  };
}

export function addCartItem(productId: string, skuId?: string, quantity = 1): CartItem {
  const product = products.find((item) => item.productId === productId);
  if (!product) {
    throw new Error('product not found');
  }
  const existing = cartItems.find((item) => item.productId === productId && item.skuId === skuId);
  if (existing) {
    existing.quantity += quantity;
    return existing;
  }
  const item: CartItem = {
    cartItemId: `cart_${Date.now()}`,
    productId,
    skuId,
    name: product.name,
    imageUrl: product.imageUrl,
    price: product.price,
    quantity,
    selected: true,
    stockStatus: product.stockStatus,
    merchantId: product.merchantId,
    merchantName: product.merchantName
  };
  cartItems = [...cartItems, item];
  return item;
}

export function updateCartItem(cartItemId: string, patch: Partial<Pick<CartItem, 'quantity' | 'selected'>>): Cart {
  cartItems = cartItems.map((item) => (item.cartItemId === cartItemId ? { ...item, ...patch } : item));
  return getCart();
}

export function removeCartItem(cartItemId: string): Cart {
  cartItems = cartItems.filter((item) => item.cartItemId !== cartItemId);
  return getCart();
}

export function createMockSession(): AgentSession {
  return {
    sessionId: `sess_${Date.now()}`,
    title: 'AI 导购',
    turns: []
  };
}

export function buildMockAgentEvents(sessionId: string, content: string): AgentSseEvent[] {
  const runId = `run_${Date.now()}`;
  const messageId = `msg_${Date.now()}`;
  const isMouse = content.includes('鼠标') || content.includes('办公');
  const recommended = isMouse ? products[2] : products[0];
  const productBlock: AgentBlock = {
    type: 'product_card',
    product: {
      productId: recommended.productId,
      skuId: recommended.skuId,
      merchantId: recommended.merchantId,
      merchantName: recommended.merchantName,
      categoryId: recommended.categoryId,
      name: recommended.name,
      brand: recommended.brand,
      imageUrl: recommended.imageUrl,
      price: recommended.price,
      marketPrice: recommended.marketPrice,
      stockStatus: recommended.stockStatus,
      tags: recommended.tags,
      sellingPoints: recommended.sellingPoints,
      recommendReason: isMouse ? '静音、无线、握持舒适，更适合办公和宿舍。' : '预算内，拍照和抓拍能力更贴近拍娃场景。',
      riskNotes: isMouse ? ['不适合高强度电竞'] : ['续航不是同价位最强']
    }
  };
  const answer = isMouse
    ? '如果主要办公，我建议先看静音、无线连接和握持舒适度。这款鼠标价格亲民，适合宿舍、办公室等低噪音环境。'
    : '预算 3000 以内拍娃，我会优先考虑对焦速度、抓拍稳定性和存储空间。下面这款更适合作为第一候选。';

  return [
    { type: 'message_start', run_id: runId, session_id: sessionId, user_message_id: messageId },
    { type: 'status', run_id: runId, stage: 'query_rewrite', text: '正在理解你的需求' },
    { type: 'text_delta', run_id: runId, delta: answer.slice(0, 24) },
    { type: 'text_delta', run_id: runId, delta: answer.slice(24) },
    { type: 'block_delta', run_id: runId, block: productBlock },
    {
      type: 'block_delta',
      run_id: runId,
      block: {
        type: 'citation',
        citation: {
          chunkId: 'ck_mock_001',
          title: '商品详情与导购资料',
          snippet: '商品卖点、适用人群和风险提示来自商品资料与知识库。',
          source: 'mock'
        }
      }
    },
    {
      type: 'followups',
      run_id: runId,
      questions: isMouse ? ['你更偏办公还是游戏？', '需要蓝牙双模吗？'] : ['你更重视拍照还是续航？', '是否需要 256GB 以上存储？']
    },
    { type: 'message_end', run_id: runId, final_output: { text: answer, blocks: [productBlock] } }
  ];
}
