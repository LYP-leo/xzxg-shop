import { consumeAgentStream } from './stream_state.mjs';
import { createReadStream } from 'node:fs';
import { mkdir, writeFile } from 'node:fs/promises';
import { createInterface } from 'node:readline';
import path from 'node:path';

export const baseURL = process.env.API_BASE_URL ?? 'http://127.0.0.1:8080/api/v1';
export const reportRoot = process.env.EVAL_REPORT_DIR ?? path.resolve('quality/reports');

export async function loadJSONL(file) {
  const items = [];
  for await (const line of createInterface({ input: createReadStream(file), crlfDelay: Infinity })) {
    if (!line.trim() || line.trim().startsWith('#')) continue;
    items.push(JSON.parse(line));
  }
  return items;
}

export async function login(username, password) {
  const response = await fetch(`${baseURL}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  });
  if (!response.ok) {
    throw new Error(`login failed for ${username}: ${response.status} ${await response.text()}`);
  }
  return (await response.json()).token;
}

export function authHeaders(token) {
  return { Authorization: `Bearer ${token}` };
}

export async function writeJSONReport(kind, payload) {
  const runID = process.env.EVAL_RUN_ID ?? `${kind}_${new Date().toISOString().replace(/[-:]/g, '').replace(/\..+/, '')}`;
  const reportDir = path.join(reportRoot, runID);
  await mkdir(reportDir, { recursive: true });
  const file = path.join(reportDir, `${kind}.json`);
  await writeFile(file, JSON.stringify(payload, null, 2));
  return file;
}

export async function requestJSON(pathname, options = {}) {
  const response = await fetch(`${baseURL}${pathname}`, options);
  if (!response.ok) {
    throw new Error(`${pathname} failed: ${response.status} ${await response.text()}`);
  }
  return response.json();
}

export async function streamAgentAnswer(token, sessionID, content, attachments = []) {
  const response = await fetch(`${baseURL}/agent/sessions/${sessionID}/messages:stream`, {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({
      client_message_id: `eval_${Date.now()}_${Math.random().toString(16).slice(2)}`,
      content,
      attachments
    })
  });
  if (!response.ok || !response.body) {
    throw new Error(`stream failed: ${response.status} ${await response.text()}`);
  }

  return consumeAgentStream(response.body);
}
