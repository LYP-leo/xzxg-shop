import type { AgentBlock, AgentTurn } from '../../types/agent';
import type { ProductCard as ProductCardType } from '../../types/product';
import { ProductCard } from '../product/ProductCard';

type Props = {
  turns: AgentTurn[];
  onFollowup: (question: string) => void;
  onOpenProduct: (productId: string) => void;
  onAddToCart: (product: ProductCardType) => void;
  onNavigate: (target: string) => void;
};

export function ChatMessageList({ turns, onFollowup, onOpenProduct, onAddToCart, onNavigate }: Props) {
  if (!turns.length) {
    return (
      <div className="empty-state">
        <h2>小猪小狗 AI 导购</h2>
        <p>输入预算、使用场景或上传商品图片，Agent 会结合商品与知识库给出建议。</p>
      </div>
    );
  }

  return (
    <div className="message-list">
      {turns.map((turn) => (
        <section className="turn" key={turn.userMessageId}>
          <div className="message message--user">{turn.userContent}</div>
          <div className="message message--agent">
            {turn.statusText ? <div className="agent-status">{turn.statusText}</div> : null}
            <div className="streaming-text">
              <MarkdownText content={turn.text} />
            </div>
            <div className="block-list">
              {turn.blocks.map((block, index) => (
                <AgentBlockView
                  block={block}
                  // Blocks can repeat with same backing id while streaming.
                  key={`${block.type}-${index}`}
                  onOpenProduct={onOpenProduct}
                  onAddToCart={onAddToCart}
                  onNavigate={onNavigate}
                />
              ))}
            </div>
            {turn.followups.length ? (
              <div className="followups">
                {turn.followups.map((question) => (
                  <button className="chip" key={question} onClick={() => onFollowup(question)}>
                    {question}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        </section>
      ))}
    </div>
  );
}

function AgentBlockView({
  block,
  onOpenProduct,
  onAddToCart,
  onNavigate
}: {
  block: AgentBlock;
  onOpenProduct: (productId: string) => void;
  onAddToCart: (product: ProductCardType) => void;
  onNavigate: (target: string) => void;
}) {
  if (block.type === 'markdown') {
    return <MarkdownText content={block.content} />;
  }
  if (block.type === 'product_card') {
    return <ProductCard product={block.product} onOpen={onOpenProduct} onAddToCart={onAddToCart} />;
  }
  if (block.type === 'citation') {
    return (
      <details className="citation">
        <summary>{block.citation.title}</summary>
        <p>{block.citation.snippet}</p>
      </details>
    );
  }
  if (block.type === 'comparison_table') {
    return (
      <table className="compare-table">
        <thead>
          <tr>
            {block.columns.map((column) => (
              <th key={column}>{column}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {block.rows.map((row) => (
            <tr key={row.productId}>
              {row.values.map((value, index) => (
                <td key={`${row.productId}-${index}`}>{value}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    );
  }
  if (block.type === 'cart_state') {
    return (
      <div className="cart-state-block">
        <div className="block-title">购物车</div>
        {block.cart.items.length ? (
          <ul>
            {block.cart.items.map((item, index) => (
              <li key={item.cartItemId}>
                <span>{index + 1}. {item.name}</span>
                <strong>x{item.quantity}</strong>
                <em>¥{item.price}</em>
              </li>
            ))}
          </ul>
        ) : (
          <p>购物车已清空。</p>
        )}
        <div className="cart-state-summary">
          已选 {block.cart.summary.selectedCount} 件，应付 ¥{block.cart.summary.payAmount}
        </div>
      </div>
    );
  }
  if (block.type === 'order_summary') {
    return (
      <div className="order-summary-block">
        <div className="block-title">订单已创建</div>
        {block.orders.map((order) => (
          <div className="order-summary-item" key={order.order_id}>
            <span>{order.merchant_name}</span>
            <strong>¥{order.total_amount}</strong>
            <small>{order.status}</small>
          </div>
        ))}
      </div>
    );
  }
  if (block.type === 'action') {
    if (block.action.name === 'navigate') {
      return (
        <div className="action-block">
          {block.message ? <p>{block.message}</p> : null}
          <button className="button" onClick={() => onNavigate(block.action.target)}>
            {block.action.label ?? '打开'}
          </button>
        </div>
      );
    }
    return <div className="action-block">{block.message}</div>;
  }
  return <div className="warning">{block.message}</div>;
}

function MarkdownText({ content }: { content: string }) {
  if (!content.trim()) {
    return null;
  }
  return (
    <>
      {content.split('\n').map((line, index) => {
        const normalized = line.trim();
        if (!normalized) {
          return <br key={index} />;
        }
        if (normalized.startsWith('### ')) {
          return <h4 key={index}>{renderInlineMarkdown(normalized.slice(4))}</h4>;
        }
        if (normalized.startsWith('## ')) {
          return <h3 key={index}>{renderInlineMarkdown(normalized.slice(3))}</h3>;
        }
        if (normalized.startsWith('# ')) {
          return <h3 key={index}>{renderInlineMarkdown(normalized.slice(2))}</h3>;
        }
        if (normalized.startsWith('- ') || normalized.startsWith('* ')) {
          return <p key={index} className="markdown-list-line">• {renderInlineMarkdown(normalized.slice(2))}</p>;
        }
        return <p key={index}>{renderInlineMarkdown(normalized)}</p>;
      })}
    </>
  );
}

function renderInlineMarkdown(text: string) {
  const parts = text.split(/(\*\*[^*]+\*\*)/g);
  return parts.map((part, index) => {
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={index}>{part.slice(2, -2)}</strong>;
    }
    return <span key={index}>{part}</span>;
  });
}
