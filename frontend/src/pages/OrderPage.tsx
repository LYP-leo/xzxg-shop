import { useEffect, useState } from 'react';
import { listUserOrders } from '../api/order';
import type { Order } from '../types/order';

export function OrderPage() {
  const [orders, setOrders] = useState<Order[]>([]);

  useEffect(() => {
    listUserOrders().then(setOrders);
  }, []);

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>我的订单</h1>
          <p>查看从购物车提交后的订单状态和商品明细。</p>
        </div>
      </header>
      <OrderTable orders={orders} />
    </section>
  );
}

export function OrderTable({ orders }: { orders: Order[] }) {
  if (!orders.length) {
    return <div className="empty-state">暂无订单。</div>;
  }
  return (
    <section className="panel">
      <table className="data-table">
        <thead>
          <tr>
            <th>订单</th>
            <th>商家</th>
            <th>状态</th>
            <th>金额</th>
            <th>商品</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((order) => (
            <tr key={order.order_id}>
              <td>{order.order_id}</td>
              <td>{order.merchant_name}</td>
              <td>{orderStatusText(order.status)}</td>
              <td>¥{order.total_amount}</td>
              <td>{order.items.map((item) => `${item.name} x${item.quantity}`).join('，')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

export function orderStatusText(status: Order['status']) {
  if (status === 'pending_ship') return '待发货';
  if (status === 'shipped') return '已发货';
  if (status === 'completed') return '已完成';
  return '已取消';
}
