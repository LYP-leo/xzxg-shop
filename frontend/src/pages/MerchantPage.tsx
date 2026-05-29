import { useEffect, useState } from 'react';
import {
  createMerchantProduct,
  deleteMerchantProduct,
  KnowledgeDocument,
  listMerchantDocuments,
  listMerchantOrders,
  MerchantProductInput,
  updateMerchantOrderStatus,
  updateMerchantProduct,
  uploadMerchantDocument
} from '../api/merchant';
import { listProducts } from '../api/product';
import type { Account } from '../types/auth';
import type { Order } from '../types/order';
import type { ProductCard } from '../types/product';
import { orderStatusText } from './OrderPage';

type MerchantPageProps = {
  account: Account;
  token: string;
};

const emptyProductForm: MerchantProductInput = {
  name: '',
  brand: '',
  category_id: 'c_phone',
  image_url: '',
  price: '',
  market_price: '',
  stock_quantity: 10,
  stock_status: 'in_stock',
  tags: [],
  selling_points: [],
  recommend_reason: '',
  risk_notes: [],
  description: ''
};

export function MerchantPage({ account, token }: MerchantPageProps) {
  const [products, setProducts] = useState<ProductCard[]>([]);
  const [documents, setDocuments] = useState<KnowledgeDocument[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [editingProductId, setEditingProductId] = useState<string>();
  const [productForm, setProductForm] = useState<MerchantProductInput>(emptyProductForm);
  const [documentForm, setDocumentForm] = useState({ title: '', doc_type: 'product_detail', content: '' });
  const [status, setStatus] = useState('');

  useEffect(() => {
    refreshMerchantData();
  }, [token]);

  async function refreshMerchantData() {
    const [nextProducts, nextDocuments, nextOrders] = await Promise.all([listProducts(), listMerchantDocuments(token), listMerchantOrders(token)]);
    setProducts(nextProducts.filter((product) => product.merchantId === account.merchant_id));
    setDocuments(nextDocuments);
    setOrders(nextOrders);
  }

  function editProduct(product: ProductCard) {
    setEditingProductId(product.productId);
    setProductForm({
      name: product.name,
      brand: product.brand,
      category_id: product.categoryId,
      image_url: product.imageUrl,
      price: product.price,
      market_price: product.marketPrice ?? product.price,
      stock_quantity: 10,
      stock_status: product.stockStatus,
      tags: product.tags,
      selling_points: product.sellingPoints,
      recommend_reason: product.recommendReason ?? '',
      risk_notes: product.riskNotes ?? [],
      description: product.recommendReason ?? ''
    });
  }

  async function saveProduct() {
    setStatus('');
    if (!productForm.name.trim()) {
      setStatus('商品名称不能为空');
      return;
    }
    const payload = normalizeForm(productForm);
    if (editingProductId) {
      await updateMerchantProduct(token, editingProductId, payload);
      setStatus('商品信息已更新');
    } else {
      await createMerchantProduct(token, payload);
      setStatus('商品已新增');
    }
    setEditingProductId(undefined);
    setProductForm(emptyProductForm);
    await refreshMerchantData();
  }

  async function uploadDocument() {
    setStatus('');
    if (!documentForm.title.trim() || !documentForm.content.trim()) {
      setStatus('资料标题和内容不能为空');
      return;
    }
    await uploadMerchantDocument(token, documentForm);
    setStatus('商品资料已上传并写入知识库');
    setDocumentForm({ title: '', doc_type: 'product_detail', content: '' });
    await refreshMerchantData();
  }

  async function removeProduct(productId: string) {
    await deleteMerchantProduct(token, productId);
    setStatus('商品已删除');
    await refreshMerchantData();
  }

  async function shipOrder(orderId: string) {
    await updateMerchantOrderStatus(token, orderId, 'shipped');
    setStatus('订单已标记发货');
    await refreshMerchantData();
  }

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>商家工作台</h1>
          <p>{account.display_name} 正在维护商品、知识素材和导购表现。</p>
        </div>
      </header>
      <div className="metric-grid">
        <section className="metric-card">
          <span>在售商品</span>
          <strong>{products.length}</strong>
        </section>
        <section className="metric-card">
          <span>知识素材</span>
          <strong>{documents.length}</strong>
        </section>
        <section className="metric-card">
          <span>订单</span>
          <strong>{orders.length}</strong>
        </section>
      </div>
      {status ? <p className="notice">{status}</p> : null}
      <div className="merchant-grid">
        <section className="panel">
          <h2>{editingProductId ? '编辑商品' : '新增商品'}</h2>
          <div className="form-grid">
            <label>
              商品名称
              <input value={productForm.name} onChange={(event) => setProductForm({ ...productForm, name: event.target.value })} />
            </label>
            <label>
              品牌
              <input value={productForm.brand} onChange={(event) => setProductForm({ ...productForm, brand: event.target.value })} />
            </label>
            <label>
              类目
              <select value={productForm.category_id} onChange={(event) => setProductForm({ ...productForm, category_id: event.target.value })}>
                <option value="c_phone">手机</option>
                <option value="c_mouse">鼠标</option>
              </select>
            </label>
            <label>
              价格
              <input value={productForm.price} onChange={(event) => setProductForm({ ...productForm, price: event.target.value })} />
            </label>
            <label>
              市场价
              <input value={productForm.market_price} onChange={(event) => setProductForm({ ...productForm, market_price: event.target.value })} />
            </label>
            <label>
              库存
              <input
                type="number"
                value={productForm.stock_quantity}
                onChange={(event) => setProductForm({ ...productForm, stock_quantity: Number(event.target.value) })}
              />
            </label>
            <label className="form-span">
              图片 URL
              <input value={productForm.image_url} onChange={(event) => setProductForm({ ...productForm, image_url: event.target.value })} />
            </label>
            <label className="form-span">
              标签
              <input value={productForm.tags.join('，')} onChange={(event) => setProductForm({ ...productForm, tags: splitWords(event.target.value) })} />
            </label>
            <label className="form-span">
              卖点
              <input
                value={productForm.selling_points.join('，')}
                onChange={(event) => setProductForm({ ...productForm, selling_points: splitWords(event.target.value) })}
              />
            </label>
            <label className="form-span">
              推荐理由
              <textarea value={productForm.recommend_reason} onChange={(event) => setProductForm({ ...productForm, recommend_reason: event.target.value })} />
            </label>
            <label className="form-span">
              商品描述
              <textarea value={productForm.description} onChange={(event) => setProductForm({ ...productForm, description: event.target.value })} />
            </label>
          </div>
          <div className="panel-actions">
            {editingProductId ? (
              <button
                className="button button--ghost"
                onClick={() => {
                  setEditingProductId(undefined);
                  setProductForm(emptyProductForm);
                }}
              >
                取消编辑
              </button>
            ) : null}
            <button className="button" onClick={saveProduct}>
              保存商品
            </button>
          </div>
        </section>
        <section className="panel">
          <h2>上传商品资料</h2>
          <div className="form-grid form-grid--single">
            <label>
              标题
              <input value={documentForm.title} onChange={(event) => setDocumentForm({ ...documentForm, title: event.target.value })} />
            </label>
            <label>
              类型
              <select value={documentForm.doc_type} onChange={(event) => setDocumentForm({ ...documentForm, doc_type: event.target.value })}>
                <option value="product_detail">商品详情</option>
                <option value="promotion">营销规则</option>
                <option value="after_sale">售后政策</option>
              </select>
            </label>
            <label>
              内容
              <textarea value={documentForm.content} onChange={(event) => setDocumentForm({ ...documentForm, content: event.target.value })} />
            </label>
          </div>
          <div className="panel-actions">
            <button className="button" onClick={uploadDocument}>
              上传资料
            </button>
          </div>
        </section>
      </div>
      <section className="panel">
        <h2>商品运营</h2>
        <table className="data-table">
          <thead>
            <tr>
              <th>商品</th>
              <th>价格</th>
              <th>库存</th>
              <th>导购卖点</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {products.map((product) => (
              <tr key={product.productId}>
                <td>{product.name}</td>
                <td>¥{product.price}</td>
                <td>{product.stockStatus}</td>
                <td>{product.sellingPoints.join(' / ')}</td>
                <td>
                  <button className="button button--ghost" onClick={() => editProduct(product)}>
                    编辑
                  </button>
                  <button className="button button--ghost" onClick={() => removeProduct(product.productId)}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section className="panel">
        <h2>订单履约</h2>
        <table className="data-table">
          <thead>
            <tr>
              <th>订单</th>
              <th>状态</th>
              <th>金额</th>
              <th>商品</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((order) => (
              <tr key={order.order_id}>
                <td>{order.order_id}</td>
                <td>{orderStatusText(order.status)}</td>
                <td>¥{order.total_amount}</td>
                <td>{order.items.map((item) => `${item.name} x${item.quantity}`).join('，')}</td>
                <td>
                  <button className="button button--ghost" disabled={order.status !== 'pending_ship'} onClick={() => shipOrder(order.order_id)}>
                    发货
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section className="panel">
        <h2>已上传资料</h2>
        <table className="data-table">
          <thead>
            <tr>
              <th>标题</th>
              <th>类型</th>
              <th>状态</th>
              <th>Chunk</th>
            </tr>
          </thead>
          <tbody>
            {documents.map((document) => (
              <tr key={document.document_id}>
                <td>{document.title}</td>
                <td>{document.doc_type}</td>
                <td>{document.status}</td>
                <td>{document.chunk_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </section>
  );
}

function splitWords(value: string) {
  return value
    .split(/[，,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeForm(input: MerchantProductInput): MerchantProductInput {
  return {
    ...input,
    brand: input.brand || '未设置',
    price: input.price || '0.00',
    market_price: input.market_price || input.price || '0.00',
    image_url: input.image_url || 'https://images.unsplash.com/photo-1516321318423-f06f85e504b3?auto=format&fit=crop&w=640&q=80',
    recommend_reason: input.recommend_reason || input.description,
    description: input.description || input.recommend_reason
  };
}
