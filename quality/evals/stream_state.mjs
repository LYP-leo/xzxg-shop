// Canonical evaluation view of the agent SSE protocol. Keep renderable cards
// from both event variants; transport EOF alone never means a successful run.
export class AgentStreamError extends Error {
  constructor(message, output) {
    super(message);
    this.name = 'AgentStreamError';
    this.output = output;
  }
}

export function createAgentState() {
  return { answer: '', run_id: '', trace_id: '', blocks: [], events: [], errors: [], started: false, complete: false };
}

export function reduceAgentEvent(state, event) {
  if (!event || typeof event.type !== 'string') throw new AgentStreamError('Missing event type', state);
  if (state.complete) throw new AgentStreamError('Event after message_end', state);
  if (event.type === 'message_start') {
    if (state.started) throw new AgentStreamError('Duplicate message_start', state);
    if (!event.run_id) throw new AgentStreamError('Missing run_id', state);
    state.started = true;
    state.run_id = event.run_id;
    state.trace_id = event.trace_id ?? '';
  } else if (!state.started) {
    throw new AgentStreamError('Event before message_start', state);
  }
  if (event.run_id && event.run_id !== state.run_id) throw new AgentStreamError('Mixed run IDs', state);
  state.events.push(event);
  if (event.type === 'text_delta') state.answer += event.delta ?? '';
  if (event.type === 'block_delta' && event.block) state.blocks.push(event.block);
  if (event.type === 'content_delta' && event.part) {
    if (event.part.type === 'text') state.answer += event.part.text ?? event.part.delta ?? '';
    else state.blocks.push(event.part);
  }
  if (event.type === 'error') state.errors.push({ code: event.code, message: event.message });
  if (event.type === 'message_end') state.complete = true;
  return state;
}

export function finishAgentState(state) {
  if (!state.complete) throw new AgentStreamError('Stream ended before message_end', state);
  if (state.errors.length) throw new AgentStreamError(`Agent failed: ${state.errors.map(e => e.code ?? e.message).join(', ')}`, state);
  return state;
}

export async function consumeAgentStream(body) {
  const state = createAgentState();
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  const parseFrame = frame => {
    const data = [];
    let eventType = '';
    for (const line of frame.split(/\r\n|\n|\r/)) {
      if (line.startsWith(':')) continue;
      const colon = line.indexOf(':');
      const field = colon < 0 ? line : line.slice(0, colon);
      const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '');
      if (field === 'data') data.push(value);
      if (field === 'event') eventType = value;
    }
    if (!data.length) return;
    let event;
    try { event = JSON.parse(data.join('\n')); }
    catch { throw new AgentStreamError('Invalid event JSON', state); }
    if (!event || typeof event !== 'object' || Array.isArray(event)) throw new AgentStreamError('Invalid event payload', state);
    if (eventType && event.type && event.type !== eventType) throw new AgentStreamError('Conflicting event types', state);
    reduceAgentEvent(state, { ...event, type: event.type ?? eventType });
  };
  const drain = () => {
    let match;
    while ((match = /\r\n\r\n|\n\n|\r\r/.exec(buffer))) {
      parseFrame(buffer.slice(0, match.index));
      buffer = buffer.slice(match.index + match[0].length);
    }
  };
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      drain();
    }
    buffer += decoder.decode();
    drain();
    // A final non-delimited data frame has not been dispatched by SSE and must
    // not be silently interpreted as a successful terminal event.
    if (buffer.split(/\r\n|\n|\r/).some(line => line.trim() && !line.startsWith(':'))) throw new AgentStreamError('Truncated SSE frame', state);
    return finishAgentState(state);
  } catch (error) {
    await reader.cancel().catch(() => {});
    if (error instanceof AgentStreamError) throw error;
    throw new AgentStreamError(`Stream transport failed: ${error.message}`, state);
  } finally { reader.releaseLock(); }
}
