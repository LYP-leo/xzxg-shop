import { useState } from 'react';
import { AdminPage } from './pages/AdminPage';
import { AgentSessionPage } from './pages/AgentSessionPage';
import { CartPage } from './pages/CartPage';
import { ProductDetailPage } from './pages/ProductDetailPage';
import { ProductListPage } from './pages/ProductListPage';

type Route = 'agent' | 'products' | 'cart' | 'admin' | 'detail';

export function App() {
  const [route, setRoute] = useState<Route>('agent');
  const [selectedProductId, setSelectedProductId] = useState<string>();
  const [cartRefreshToken, setCartRefreshToken] = useState(0);
  const [agentSeedQuestion, setAgentSeedQuestion] = useState<string>();

  function openProduct(productId: string) {
    setSelectedProductId(productId);
    setRoute('detail');
  }

  function askAgent(question: string) {
    setAgentSeedQuestion(question);
    setRoute('agent');
  }

  function refreshCart() {
    setCartRefreshToken((value) => value + 1);
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">AI</span>
          <div>
            <strong>小猪小狗导购</strong>
            <small>Agent-first shop</small>
          </div>
        </div>
        <nav>
          <button className={route === 'agent' ? 'active' : ''} onClick={() => setRoute('agent')}>
            AI 导购
          </button>
          <button className={route === 'products' || route === 'detail' ? 'active' : ''} onClick={() => setRoute('products')}>
            商品
          </button>
          <button className={route === 'cart' ? 'active' : ''} onClick={() => setRoute('cart')}>
            购物车
          </button>
          <button className={route === 'admin' ? 'active' : ''} onClick={() => setRoute('admin')}>
            管理
          </button>
        </nav>
      </aside>
      <main className="main-content">
        {route === 'agent' ? (
          <AgentSessionPage
            key={agentSeedQuestion ?? 'agent'}
            initialQuestion={agentSeedQuestion}
            onOpenProduct={openProduct}
            onCartChange={refreshCart}
          />
        ) : null}
        {route === 'products' ? <ProductListPage onOpenProduct={openProduct} onCartChange={refreshCart} /> : null}
        {route === 'detail' && selectedProductId ? (
          <ProductDetailPage productId={selectedProductId} onBack={() => setRoute('products')} onAskAgent={askAgent} onCartChange={refreshCart} />
        ) : null}
        {route === 'cart' ? <CartPage refreshToken={cartRefreshToken} /> : null}
        {route === 'admin' ? <AdminPage /> : null}
      </main>
    </div>
  );
}
