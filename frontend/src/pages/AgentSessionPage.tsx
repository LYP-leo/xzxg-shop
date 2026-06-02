import { useEffect, useMemo, useRef, useState } from 'react';
import { cancelAgentRun, createAgentSession, streamAgentMessage } from '../api/agent';
import { addToCart } from '../api/cart';
import { ChatInputBar } from '../components/chat/ChatInputBar';
import { ChatMessageList } from '../components/chat/ChatMessageList';
import type { AgentSession, AgentSseEvent, AgentTurn, Attachment } from '../types/agent';
import type { ProductCard } from '../types/product';

type Props = {
  initialQuestion?: string;
  onOpenProduct: (productId: string) => void;
  onCartChange: () => void;
};

export function AgentSessionPage({ initialQuestion, onOpenProduct, onCartChange }: Props) {
  const [session, setSession] = useState<AgentSession | null>(null);
  const [activeRunId, setActiveRunId] = useState<string | null>(null);
  const hasCreatedSession = useRef(false);
  const hasSentInitialQuestion = useRef(false);

  useEffect(() => {
    if (hasCreatedSession.current) return;
    hasCreatedSession.current = true;
    createAgentSession().then(setSession);
  }, []);

  useEffect(() => {
    if (!session || !initialQuestion || hasSentInitialQuestion.current) return;
    hasSentInitialQuestion.current = true;
    void sendMessage(initialQuestion);
  }, [initialQuestion, session]);

  const isStreaming = useMemo(() => Boolean(activeRunId), [activeRunId]);

  async function sendMessage(content: string, attachments: Attachment[] = []) {
    if (!session || activeRunId) return;
    const optimisticTurn: AgentTurn = {
      userMessageId: `local_${Date.now()}`,
      userContent: content,
      attachments,
      status: 'streaming',
      text: '',
      blocks: [],
      followups: [],
      createdAt: new Date().toISOString()
    };
    setSession({ ...session, turns: [...session.turns, optimisticTurn] });

    await streamAgentMessage({
      sessionId: session.sessionId,
      content,
      attachments,
      onEvent: handleEvent
    });
  }

  function handleEvent(event: AgentSseEvent) {
    if (event.type === 'message_start') {
      setActiveRunId(event.run_id);
      setSession((current) => replaceLastTurn(current, { runId: event.run_id, userMessageId: event.user_message_id }));
      return;
    }
    if (event.type === 'status') {
      setSession((current) => updateTurnByRun(current, event.run_id, { statusText: event.text }));
      return;
    }
    if (event.type === 'text_delta') {
      setSession((current) =>
        updateTurnByRun(current, event.run_id, (turn) => ({
          ...turn,
          text: `${turn.text}${event.delta}`,
          statusText: undefined
        }))
      );
      return;
    }
    if (event.type === 'block_delta') {
      setSession((current) =>
        updateTurnByRun(current, event.run_id, (turn) => ({
          ...turn,
          blocks: [...turn.blocks, event.block]
        }))
      );
      return;
    }
    if (event.type === 'followups') {
      setSession((current) => updateTurnByRun(current, event.run_id, { followups: event.questions }));
      return;
    }
    if (event.type === 'message_end') {
      setSession((current) => updateTurnByRun(current, event.run_id, { status: 'completed', statusText: undefined }));
      setActiveRunId(null);
      return;
    }
    if (event.type === 'error') {
      const runId = event.run_id ?? activeRunId;
      setSession((current) =>
        runId
          ? updateTurnByRun(current, runId, { status: 'failed', statusText: event.message })
          : replaceLastTurn(current, { status: 'failed', statusText: event.message })
      );
      setActiveRunId(null);
    }
  }

  async function addProduct(product: ProductCard) {
    await addToCart({
      productId: product.productId,
      skuId: product.skuId,
      quantity: 1,
      source: 'agent_recommendation'
    });
    onCartChange();
  }

  async function cancelRun() {
    if (!activeRunId) return;
    await cancelAgentRun(activeRunId);
    setSession((current) => updateTurnByRun(current, activeRunId, { status: 'canceled', statusText: '已停止生成' }));
    setActiveRunId(null);
  }

  return (
    <section className="agent-page">
      <header className="page-header">
        <div>
          <h1>AI 导购</h1>
          <p>基于商品库和知识库生成可解释推荐。</p>
        </div>
        <span className="pill">{session?.sessionId ?? 'creating'}</span>
      </header>
      <ChatMessageList
        turns={session?.turns ?? []}
        onFollowup={sendMessage}
        onOpenProduct={onOpenProduct}
        onAddToCart={addProduct}
      />
      <ChatInputBar disabled={isStreaming || !session} onSend={sendMessage} onCancel={cancelRun} />
    </section>
  );
}

function replaceLastTurn(session: AgentSession | null, patch: Partial<AgentTurn>): AgentSession | null {
  if (!session) return session;
  const turns = [...session.turns];
  const last = turns[turns.length - 1];
  if (!last) return session;
  turns[turns.length - 1] = { ...last, ...patch };
  return { ...session, turns };
}

function updateTurnByRun(
  session: AgentSession | null,
  runId: string,
  patch: Partial<AgentTurn> | ((turn: AgentTurn) => AgentTurn)
): AgentSession | null {
  if (!session) return session;
  return {
    ...session,
    turns: session.turns.map((turn) => {
      if (turn.runId !== runId) return turn;
      return typeof patch === 'function' ? patch(turn) : { ...turn, ...patch };
    })
  };
}
