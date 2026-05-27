import type { ProductCard } from './product';
import type { Cart } from './cart';
import type { Order } from './order';

export type Attachment = {
  attachmentId: string;
  type: 'image' | 'file';
  url?: string;
  name?: string;
  objectKey?: string;
};

export type Citation = {
  chunkId: string;
  title: string;
  snippet: string;
  source?: string;
};

export type AgentBlock =
  | { type: 'markdown'; content: string }
  | { type: 'product_card'; product: ProductCard }
  | { type: 'comparison_table'; columns: string[]; rows: Array<{ productId: string; values: string[] }> }
  | { type: 'cart_state'; cart: Cart }
  | { type: 'order_summary'; orders: Order[] }
  | { type: 'citation'; citation: Citation }
  | { type: 'action'; message?: string; action: { name: string; target: string; label?: string } }
  | { type: 'warning'; code: string; message: string };

export type AgentTurn = {
  userMessageId: string;
  userContent: string;
  attachments: Attachment[];
  runId?: string;
  status: 'idle' | 'streaming' | 'completed' | 'failed' | 'canceled';
  statusText?: string;
  text: string;
  blocks: AgentBlock[];
  followups: string[];
  createdAt: string;
};

export type AgentSession = {
  sessionId: string;
  title: string;
  summary?: string;
  turns: AgentTurn[];
};

export type AgentSseEvent =
  | { type: 'message_start'; run_id: string; session_id: string; user_message_id: string }
  | { type: 'status'; run_id: string; stage: string; text: string }
  | { type: 'text_delta'; run_id: string; delta: string }
  | { type: 'block_delta'; run_id: string; block: AgentBlock }
  | { type: 'followups'; run_id: string; questions: string[] }
  | { type: 'message_end'; run_id: string; final_output?: { text: string; blocks: AgentBlock[] } }
  | { type: 'error'; run_id?: string; code: string; message: string };
