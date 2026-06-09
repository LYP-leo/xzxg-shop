import { useState } from 'react';
import { clearSession, loadSession } from './api/auth';
import { AdminPage } from './pages/AdminPage';
import { LoginPage } from './pages/LoginPage';
import { MerchantPage } from './pages/MerchantPage';
import { ProductDetailPage } from './pages/ProductDetailPage';
import { ProductListPage } from './pages/ProductListPage';
import type { AuthSession } from './types/auth';

type Route =
  | 'products'
  | 'merchant'
  | 'admin'
  | 'adminPrompts'
  | 'adminDebug'
  | 'adminEval'
  | 'adminRisk'
  | 'detail';

export function App() {
  const [session, setSession] = useState<AuthSession | undefined>(() => loadSession());
  const [route, setRoute] = useState<Route>(() => defaultRoute(loadSession()?.account.role));
  const [selectedProductId, setSelectedProductId] = useState<string>();

  function openProduct(productId: string) {
    setSelectedProductId(productId);
    setRoute('detail');
  }

  function handleLogin(nextSession: AuthSession) {
    setSession(nextSession);
    setRoute(defaultRoute(nextSession.account.role));
  }

  function logout() {
    clearSession();
    setSession(undefined);
    setRoute('admin');
  }

  if (!session) {
    return <LoginPage onLogin={handleLogin} />;
  }

  const account = session.account;
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
              <button className={route === 'adminPrompts' ? 'active' : ''} onClick={() => setRoute('adminPrompts')}>
                Prompt 管理
              </button>
              <button className={route === 'adminDebug' ? 'active' : ''} onClick={() => setRoute('adminDebug')}>
                调试观测
              </button>
              <button className={route === 'adminEval' ? 'active' : ''} onClick={() => setRoute('adminEval')}>
                质量评测
              </button>
              <button className={route === 'adminRisk' ? 'active' : ''} onClick={() => setRoute('adminRisk')}>
                风控管理
              </button>
              <button className={route === 'products' || route === 'detail' ? 'active' : ''} onClick={() => setRoute('products')}>
                商品巡检
              </button>
            </>
          ) : null}
        </nav>
      </aside>
      <main className="main-content">
        {route === 'products' ? <ProductListPage onOpenProduct={openProduct} /> : null}
        {route === 'detail' && selectedProductId ? (
          <ProductDetailPage
            productId={selectedProductId}
            onBack={() => setRoute('products')}
          />
        ) : null}
        {route === 'merchant' && isMerchant ? <MerchantPage account={account} token={session.token} /> : null}
        {route === 'admin' && isAdmin ? <AdminPage token={session.token} view="platform" /> : null}
        {route === 'adminPrompts' && isAdmin ? <AdminPage token={session.token} view="prompts" /> : null}
        {route === 'adminDebug' && isAdmin ? <AdminPage token={session.token} view="debug" /> : null}
        {route === 'adminEval' && isAdmin ? <AdminPage token={session.token} view="eval" /> : null}
        {route === 'adminRisk' && isAdmin ? <AdminPage token={session.token} view="risk" /> : null}
      </main>
    </div>
  );
}

function defaultRoute(role?: string): Route {
  if (role === 'merchant') return 'merchant';
  if (role === 'admin') return 'admin';
  return 'admin';
}

function roleLabel(role: string) {
  if (role === 'merchant') return '商家端';
  if (role === 'admin') return '管理员端';
  return '后台账号';
}
