import { addCartItem, getCart, removeCartItem, updateCartItem } from '../mock/data';
import type { Cart } from '../types/cart';
import { loadSession } from './auth';
import { requestJSON } from './http';

export async function fetchCart(): Promise<Cart> {
  try {
    return await requestJSON<Cart>('/cart', { headers: authHeaders() });
  } catch {
    return getCart();
  }
}

export async function addToCart(input: { productId: string; skuId?: string; quantity: number; source?: string }): Promise<Cart> {
  try {
    await requestJSON('/cart/items', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        product_id: input.productId,
        sku_id: input.skuId,
        quantity: input.quantity,
        source: input.source ?? 'manual'
      })
    });
    return fetchCart();
  } catch {
    addCartItem(input.productId, input.skuId, input.quantity);
    return getCart();
  }
}

export async function patchCartItem(cartItemId: string, patch: { quantity?: number; selected?: boolean }): Promise<Cart> {
  try {
    return await requestJSON<Cart>(`/cart/items/${cartItemId}`, {
      method: 'PATCH',
      headers: authHeaders(),
      body: JSON.stringify(patch)
    });
  } catch {
    return updateCartItem(cartItemId, patch);
  }
}

export async function deleteCartItem(cartItemId: string): Promise<Cart> {
  try {
    await requestJSON(`/cart/items/${cartItemId}`, { method: 'DELETE', headers: authHeaders() });
    return fetchCart();
  } catch {
    return removeCartItem(cartItemId);
  }
}

function authHeaders(): Record<string, string> {
	const token = loadSession()?.token;
	return token ? { Authorization: `Bearer ${token}` } : {};
}
