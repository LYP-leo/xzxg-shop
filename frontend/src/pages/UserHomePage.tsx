import type { Account } from '../types/auth';

type UserHomePageProps = {
  account: Account;
  onStartAgent: () => void;
  onBrowseProducts: () => void;
};

export function UserHomePage({ account, onStartAgent, onBrowseProducts }: UserHomePageProps) {
  return (
    <section>
      <header className="page-header">
        <div>
          <h1>用户端</h1>
          <p>{account.display_name} 可以使用 AI 导购、浏览商品和管理购物车。</p>
        </div>
      </header>
      <div className="user-action-grid">
        <button className="action-panel" onClick={onStartAgent}>
          <strong>AI 导购</strong>
          <span>用流式对话获取推荐、引用和商品卡片。</span>
        </button>
        <button className="action-panel" onClick={onBrowseProducts}>
          <strong>商品列表</strong>
          <span>查看从 MySQL 读取的商品、SKU 和库存信息。</span>
        </button>
      </div>
    </section>
  );
}
