import { createReadStream } from 'node:fs';
import { createInterface } from 'node:readline';
import { performance } from 'node:perf_hooks';

const baseURL = process.env.API_BASE_URL ?? 'http://127.0.0.1:8080/api/v1';
const username = process.env.EVAL_USERNAME ?? 'user';
const password = process.env.EVAL_PASSWORD ?? 'user123456';
const dataset = process.argv[2] ?? new URL('./agent_cases.jsonl', import.meta.url).pathname;

const token = await login();
const results = [];

for await (const line of createInterface({ input: createReadStream(dataset), crlfDelay: Infinity })) {
  if (!line.trim()) continue;
  const testCase = JSON.parse(line);
  if (testCase.turns) {
    results.push(await runMultiTurn(testCase));
  } else {
    results.push(await runSingleTurn(testCase, testCase.query));
  }
}

const failed = results.filter((item) => !item.ok);
for (const result of results) {
  const mark = result.ok ? 'PASS' : 'FAIL';
  console.log(`${mark} ${result.id} first_delta=${result.firstDeltaMs ?? '-'}ms blocks=${result.blocks.join(',')}`);
  for (const reason of result.reasons) console.log(`  - ${reason}`);
}

if (failed.length) {
  process.exitCode = 1;
}

async function login() {
  const response = await fetch(`${baseURL}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  });
  if (!response.ok) {
    throw new Error(`login failed: ${response.status} ${await response.text()}`);
  }
  return (await response.json()).token;
}

async function createSession() {
  const response = await fetch(`${baseURL}/agent/sessions`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ title: 'quality-eval' })
  });
  if (!response.ok) {
    throw new Error(`create session failed: ${response.status} ${await response.text()}`);
  }
  return (await response.json()).session_id;
}

async function runMultiTurn(testCase) {
  const sessionId = await createSession();
  let lastResult;
  for (const turn of testCase.turns) {
    lastResult = await streamMessage(sessionId, turn);
  }
  return assertCase(testCase, lastResult);
}

async function runSingleTurn(testCase, query) {
  const sessionId = await createSession();
  return assertCase(testCase, await streamMessage(sessionId, query, testCase.attachments ?? []));
}

async function streamMessage(sessionId, content, attachments = []) {
  const startedAt = performance.now();
  const response = await fetch(`${baseURL}/agent/sessions/${sessionId}/messages:stream`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({
      client_message_id: `eval_${Date.now()}_${Math.random().toString(16).slice(2)}`,
      content,
      attachments
    })
  });
  if (!response.ok || !response.body) {
    throw new Error(`stream failed: ${response.status} ${await response.text()}`);
  }

  let firstDeltaMs;
  let text = '';
  const blocks = [];
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const chunks = buffer.split('\n\n');
    buffer = chunks.pop() ?? '';
    for (const chunk of chunks) {
      const dataLine = chunk.split('\n').find((item) => item.startsWith('data:'));
      if (!dataLine) continue;
      const event = JSON.parse(dataLine.slice(5).trim());
      if (event.type === 'text_delta') {
        firstDeltaMs ??= Math.round(performance.now() - startedAt);
        text += event.delta;
      }
      if (event.type === 'block_delta' && event.block?.type) {
        blocks.push(event.block.type);
      }
    }
  }
  return { text, blocks, firstDeltaMs };
}

function assertCase(testCase, output) {
  const reasons = [];
  for (const phrase of testCase.must_contain ?? []) {
    if (!output.text.includes(phrase)) reasons.push(`missing text: ${phrase}`);
  }
  for (const phrase of testCase.must_not_contain ?? []) {
    if (output.text.includes(phrase)) reasons.push(`forbidden text: ${phrase}`);
  }
  for (const block of testCase.expected_blocks ?? []) {
    if (!output.blocks.includes(block)) reasons.push(`missing block: ${block}`);
  }
  if (testCase.max_first_delta_ms && output.firstDeltaMs && output.firstDeltaMs > testCase.max_first_delta_ms) {
    reasons.push(`first delta too slow: ${output.firstDeltaMs}ms > ${testCase.max_first_delta_ms}ms`);
  }
  return { id: testCase.id, ok: reasons.length === 0, reasons, ...output };
}

function authHeaders() {
  return { Authorization: `Bearer ${token}` };
}
