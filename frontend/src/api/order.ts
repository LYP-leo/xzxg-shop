import type { Order } from '../types/order';
import { loadSession } from './auth';
import { requestJSON } from './http';

export async function listUserOrders(): Promise<Order[]> {
  const data = await requestJSON<{ items: Order[] }>('/orders', {
    headers: authHeaders()
  });
  return data.items;
}

export async function checkoutCart(): Promise<Order[]> {
  const data = await requestJSON<{ items: Order[] }>('/orders:checkout', {
    method: 'POST',
    headers: authHeaders()
  });
  return data.items;
}

function authHeaders(): Record<string, string> {
  const token = loadSession()?.token;
  return token ? { Authorization: `Bearer ${token}` } : {};
}
