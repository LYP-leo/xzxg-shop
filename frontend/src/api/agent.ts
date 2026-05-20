import type { AgentSseEvent, AgentSession, Attachment } from '../types/agent';
import { loadSession } from './auth';
import { requestJSON } from './http';

export async function createAgentSession(): Promise<AgentSession> {
  const data = await requestJSON<{ session_id: string; title: string }>('/agent/sessions', {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ title: 'AI 导购', entry_source: 'chat_home' })
  });
  return {
    sessionId: data.session_id,
    title: data.title,
    turns: []
  };
}

export async function streamAgentMessage(input: {
  sessionId: string;
  content: string;
  attachments: Attachment[];
  onEvent: (event: AgentSseEvent) => void;
}): Promise<void> {
  try {
    const response = await fetch(`/api/v1/agent/sessions/${input.sessionId}/messages:stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream', ...authHeaders() },
      body: JSON.stringify({
        client_message_id: createClientMessageId(),
        content: input.content,
        attachments: input.attachments.map((item) => ({
          attachment_id: item.attachmentId,
          type: item.type,
          url: item.url
        }))
      })
    });

    if (!response.ok || !response.body) {
      input.onEvent({ type: 'error', code: 'stream_unavailable', message: 'Agent 服务暂时不可用，请稍后重试' });
      return;
    }

    await readSseStream(response.body, input.onEvent);
  } catch {
    input.onEvent({ type: 'error', code: 'network_error', message: 'Agent 连接失败，请确认后端服务已启动' });
  }
}

function createClientMessageId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `cli_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

async function readSseStream(body: ReadableStream<Uint8Array>, onEvent: (event: AgentSseEvent) => void): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const chunks = buffer.split('\n\n');
    buffer = chunks.pop() ?? '';

    for (const chunk of chunks) {
      const lines = chunk.split('\n');
      const dataLine = lines.find((line) => line.startsWith('data:'));
      if (!dataLine) continue;
      const json = dataLine.slice(5).trim();
      if (!json) continue;
      onEvent(JSON.parse(json) as AgentSseEvent);
    }
  }
}

export async function cancelAgentRun(runId: string): Promise<void> {
  try {
    await requestJSON(`/agent/runs/${runId}:cancel`, { method: 'POST', headers: authHeaders() });
  } catch {
    return;
  }
}

function authHeaders(): Record<string, string> {
	const token = loadSession()?.token;
	return token ? { Authorization: `Bearer ${token}` } : {};
}
