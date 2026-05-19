import type { AgentBlock, AgentTurn } from '../../types/agent';
import type { ProductCard as ProductCardType } from '../../types/product';
import { ProductCard } from '../product/ProductCard';

type Props = {
  turns: AgentTurn[];
  onFollowup: (question: string) => void;
  onOpenProduct: (productId: string) => void;
  onAddToCart: (product: ProductCardType) => void;
};

export function ChatMessageList({ turns, onFollowup, onOpenProduct, onAddToCart }: Props) {
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
            <p className="streaming-text">{turn.text}</p>
            <div className="block-list">
              {turn.blocks.map((block, index) => (
                <AgentBlockView
                  block={block}
                  // Blocks can repeat with same backing id while streaming.
                  key={`${block.type}-${index}`}
                  onOpenProduct={onOpenProduct}
                  onAddToCart={onAddToCart}
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
  onAddToCart
}: {
  block: AgentBlock;
  onOpenProduct: (productId: string) => void;
  onAddToCart: (product: ProductCardType) => void;
}) {
  if (block.type === 'markdown') {
    return <p>{block.content}</p>;
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
  return <div className="warning">{block.message}</div>;
}
