import { useCallback, useEffect, useRef, useState } from 'react';
import type { Order } from '../../types/order';
import type { ProductCard } from '../../types/product';

export type LoadState = {
  loading: boolean;
  error: string;
};

export function useAsyncList<T>(
  loader: () => Promise<{ items: T[]; total: number }>,
  deps: unknown[],
  enabled = true
) {
  const sequenceRef = useRef(0);
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const reload = useCallback(async () => {
    if (!enabled) return;
    const sequence = sequenceRef.current + 1;
    sequenceRef.current = sequence;
    setLoading(true);
    setError('');
    try {
      const data = await loader();
      if (sequence !== sequenceRef.current) return;
      setItems(data.items ?? []);
      setTotal(data.total ?? 0);
    } catch (err) {
      if (sequence !== sequenceRef.current) return;
      setItems([]);
      setTotal(0);
      setError(errorMessage(err));
    } finally {
      if (sequence === sequenceRef.current) {
        setLoading(false);
      }
    }
  }, deps);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { items, setItems, total, setTotal, loading, error, reload };
}

export function useAsyncValue<T>(loader: () => Promise<T>, deps: unknown[], enabled = true) {
  const sequenceRef = useRef(0);
  const [value, setValue] = useState<T>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const reload = useCallback(async () => {
    if (!enabled) return;
    const sequence = sequenceRef.current + 1;
    sequenceRef.current = sequence;
    setLoading(true);
    setError('');
    try {
      const data = await loader();
      if (sequence === sequenceRef.current) {
        setValue(data);
      }
    } catch (err) {
      if (sequence === sequenceRef.current) {
        setError(errorMessage(err));
      }
    } finally {
      if (sequence === sequenceRef.current) {
        setLoading(false);
      }
    }
  }, deps);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { value, setValue, loading, error, reload };
}

export function useNotice() {
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');

  function showNotice(message: string) {
    setNotice(message);
    setError('');
  }

  function showError(err: unknown, fallback = '操作失败') {
    setError(errorMessage(err) || fallback);
    setNotice('');
  }

  function clear() {
    setNotice('');
    setError('');
  }

  return { notice, error, showNotice, showError, clear };
}

export function PageSizeSelect({
  value,
  options = [10, 20, 50],
  onChange
}: {
  value: number;
  options?: number[];
  onChange: (value: number) => void;
}) {
  return (
    <select className="table-input table-input--compact" value={value} onChange={(event) => onChange(Number(event.target.value))}>
      {options.map((option) => (
        <option value={option} key={option}>
          {option} 条/页
        </option>
      ))}
    </select>
  );
}

export function PaginationBar({
  loading,
  page,
  totalPages,
  visibleCount,
  total,
  onPrev,
  onNext
}: {
  loading: boolean;
  page: number;
  totalPages: number;
  visibleCount: number;
  total: number;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className="pagination-bar">
      <button className="button button--ghost" disabled={loading || page <= 1} onClick={onPrev}>
        上一页
      </button>
      <span>{loading ? '加载中' : `${visibleCount} / ${total}`}</span>
      <button className="button button--ghost" disabled={loading || page >= totalPages} onClick={onNext}>
        下一页
      </button>
    </div>
  );
}

export function PanelMessage({ loading, error, empty }: { loading?: boolean; error?: string; empty?: string }) {
  if (loading) return <p className="empty-text">加载中...</p>;
  if (error) return <p className="form-error">{error}</p>;
  if (empty) return <p className="empty-text">{empty}</p>;
  return null;
}

export function StatusBadge({ status }: { status: string }) {
  return <span className={`status-badge status-badge--${status}`}>{statusText(status)}</span>;
}

export function RiskStatusSelect({
  value,
  options,
  disabled,
  onChange
}: {
  value: string;
  options: string[];
  disabled?: boolean;
  onChange: (status: string) => void;
}) {
  return (
    <select
      className="table-input table-input--compact"
      disabled={disabled}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      {options.map((status) => (
        <option value={status} key={status}>
          {statusText(status)}
        </option>
      ))}
    </select>
  );
}

export function RiskMetric({ title, value, hint }: { title: string; value: number; hint: string }) {
  return (
    <article className="risk-metric">
      <span>{title}</span>
      <strong>{value}</strong>
      <small>{hint}</small>
    </article>
  );
}

export function totalPages(total: number, pageSize: number) {
  return Math.max(1, Math.ceil(total / pageSize));
}

export function productStatus(product: ProductCard) {
  return product.status || 'active';
}

export function statusText(status: string) {
  if (status === 'active') return '正常';
  if (status === 'inactive') return '停用';
  if (status === 'risk') return '风险';
  if (status === 'deleted') return '删除';
  return status || '-';
}

export function orderStatusText(status: Order['status']) {
  if (status === 'pending_payment') return '待支付';
  if (status === 'pending_ship') return '待发货';
  if (status === 'shipped') return '已发货';
  if (status === 'completed') return '已完成';
  if (status === 'closed_timeout') return '支付超时关闭';
  if (status === 'refund_requested') return '退款中';
  if (status === 'refunded') return '已退款';
  return '已取消';
}

export function formatCount(value?: number) {
  if (value === undefined || value === null) {
    return '-';
  }
  return value.toLocaleString('zh-CN');
}

export function shortURL(value: string) {
  try {
    const url = new URL(value);
    const path = url.pathname.length > 36 ? `${url.pathname.slice(0, 34)}...` : url.pathname;
    return `${url.hostname}${path}`;
  } catch {
    return value.length > 42 ? `${value.slice(0, 39)}...` : value;
  }
}

export function formatPercent(value?: number) {
  if (value === undefined || value === null) {
    return '-';
  }
  return `${Math.round(value * 1000) / 10}%`;
}

export function formatDecimal(value: unknown) {
  const number = Number(value);
  if (Number.isNaN(number)) {
    return '-';
  }
  return String(Math.round(number * 1000) / 1000);
}

export function formatDateTime(value: string) {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString('zh-CN', { hour12: false });
}

export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error || '请求失败');
}
