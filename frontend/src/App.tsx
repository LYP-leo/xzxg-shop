import { useState } from 'react';
import { clearSession, loadSession } from './api/auth';
import { AdminPage } from './pages/AdminPage';
import { AgentSessionPage } from './pages/AgentSessionPage';
import { CartPage } from './pages/CartPage';
import { LoginPage } from './pages/LoginPage';
import { MerchantPage } from './pages/MerchantPage';
import { ProductDetailPage } from './pages/ProductDetailPage';
import { ProductListPage } from './pages/ProductListPage';
import { UserHomePage } from './pages/UserHomePage';
import type { AuthSession } from './types/auth';

type Route = 'home' | 'agent' | 'products' | 'cart' | 'merchant' | 'admin' | 'detail';

export function App() {
  const [session, setSession] = useState<AuthSession | undefined>(() => loadSession());
  const [route, setRoute] = useState<Route>(() => defaultRoute(loadSession()?.account.role));
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

  function handleLogin(nextSession: AuthSession) {
    setSession(nextSession);
    setRoute(defaultRoute(nextSession.account.role));
  }

  function logout() {
    clearSession();
    setSession(undefined);
    setRoute('home');
  }

  if (!session) {
    return <LoginPage onLogin={handleLogin} />;
  }

  const account = session.account;
  const isUser = account.role === 'user';
  const isMerchant = account.role === 'merchant';
  const isAdmin = account.role === 'admin';

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
        <div className="account-card">
          <strong>{account.display_name}</strong>
          <span>{roleLabel(account.role)}</span>
          <button onClick={logout}>退出</button>
        </div>
        <nav>
          {isUser ? (
            <>
              <button className={route === 'home' ? 'active' : ''} onClick={() => setRoute('home')}>
                首页
              </button>
              <button className={route === 'agent' ? 'active' : ''} onClick={() => setRoute('agent')}>
                AI 导购
              </button>
              <button className={route === 'products' || route === 'detail' ? 'active' : ''} onClick={() => setRoute('products')}>
                商品
              </button>
              <button className={route === 'cart' ? 'active' : ''} onClick={() => setRoute('cart')}>
                购物车
              </button>
            </>
          ) : null}
          {isMerchant ? (
            <>
              <button className={route === 'merchant' ? 'active' : ''} onClick={() => setRoute('merchant')}>
                商家工作台
              </button>
              <button className={route === 'products' || route === 'detail' ? 'active' : ''} onClick={() => setRoute('products')}>
                商品预览
              </button>
            </>
          ) : null}
          {isAdmin ? (
            <>
              <button className={route === 'admin' ? 'active' : ''} onClick={() => setRoute('admin')}>
                平台管理
              </button>
              <button className={route === 'products' || route === 'detail' ? 'active' : ''} onClick={() => setRoute('products')}>
                商品巡检
              </button>
            </>
          ) : null}
        </nav>
      </aside>
      <main className="main-content">
        {route === 'home' && isUser ? (
          <UserHomePage account={account} onStartAgent={() => setRoute('agent')} onBrowseProducts={() => setRoute('products')} />
        ) : null}
        {route === 'agent' && isUser ? (
          <AgentSessionPage
            key={agentSeedQuestion ?? 'agent'}
            initialQuestion={agentSeedQuestion}
            onOpenProduct={openProduct}
            onCartChange={refreshCart}
          />
        ) : null}
        {route === 'products' ? <ProductListPage onOpenProduct={openProduct} onCartChange={refreshCart} canAddToCart={isUser} /> : null}
        {route === 'detail' && selectedProductId ? (
          <ProductDetailPage
            productId={selectedProductId}
            onBack={() => setRoute('products')}
            onAskAgent={askAgent}
            onCartChange={refreshCart}
            canAddToCart={isUser}
            canAskAgent={isUser}
          />
        ) : null}
        {route === 'cart' && isUser ? <CartPage refreshToken={cartRefreshToken} /> : null}
        {route === 'merchant' && isMerchant ? <MerchantPage account={account} token={session.token} /> : null}
        {route === 'admin' && isAdmin ? <AdminPage token={session.token} /> : null}
      </main>
    </div>
  );
}

function defaultRoute(role?: string): Route {
  if (role === 'merchant') return 'merchant';
  if (role === 'admin') return 'admin';
  return 'home';
}

function roleLabel(role: string) {
  if (role === 'merchant') return '商家端';
  if (role === 'admin') return '管理员端';
  return '用户端';
}
