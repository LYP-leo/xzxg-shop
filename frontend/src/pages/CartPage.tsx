import { useEffect, useState } from 'react';
import { deleteCartItem, fetchCart, patchCartItem } from '../api/cart';
import { checkoutCart } from '../api/order';
import type { Cart } from '../types/cart';

type Props = {
  refreshToken: number;
};

export function CartPage({ refreshToken }: Props) {
  const [cart, setCart] = useState<Cart>();
  const [status, setStatus] = useState('');

  useEffect(() => {
    fetchCart().then(setCart);
  }, [refreshToken]);

  if (!cart) {
    return <div className="empty-state">正在加载购物车...</div>;
  }

  async function updateQuantity(cartItemId: string, quantity: number) {
    setCart(await patchCartItem(cartItemId, { quantity: Math.max(1, quantity) }));
  }

  async function toggleSelected(cartItemId: string, selected: boolean) {
    setCart(await patchCartItem(cartItemId, { selected }));
  }

  async function remove(cartItemId: string) {
    setCart(await deleteCartItem(cartItemId));
  }

  async function checkout() {
    await checkoutCart();
    setStatus('订单已提交');
    setCart(await fetchCart());
  }

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>购物车</h1>
          <p>承接 Agent 推荐后的加购动作，暂不接入真实支付。</p>
        </div>
      </header>
      <div className="cart-list">
        {status ? <p className="notice">{status}</p> : null}
        {cart.items.length ? (
          cart.items.map((item) => (
            <article className="cart-item" key={item.cartItemId}>
              <input type="checkbox" checked={item.selected} onChange={(event) => toggleSelected(item.cartItemId, event.target.checked)} />
              <img src={item.imageUrl} alt={item.name} />
              <div>
                <strong>{item.name}</strong>
                <p>{item.merchantName}</p>
              </div>
              <div>¥{item.price}</div>
              <div className="stepper">
                <button onClick={() => updateQuantity(item.cartItemId, item.quantity - 1)}>-</button>
                <span>{item.quantity}</span>
                <button onClick={() => updateQuantity(item.cartItemId, item.quantity + 1)}>+</button>
              </div>
              <button className="button button--ghost" onClick={() => remove(item.cartItemId)}>
                删除
              </button>
            </article>
          ))
        ) : (
          <div className="empty-state">购物车为空。可以从商品页或 Agent 推荐卡片加入。</div>
        )}
      </div>
      <div className="cart-summary">
        <span>已选 {cart.summary.selectedCount} 件</span>
        <strong>应付 ¥{cart.summary.payAmount}</strong>
        <button className="button" disabled={!cart.summary.selectedCount} onClick={checkout}>
          提交订单
        </button>
      </div>
    </section>
  );
}
