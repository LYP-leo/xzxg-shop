import { authHeaders, loadJSONL, login, requestJSON, streamAgentAnswer, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/agent_e2e_queries.jsonl';
const username = process.env.EVAL_USERNAME ?? 'user';
const password = process.env.EVAL_PASSWORD ?? 'user123456';

const token = await login(username, password);
const cases = await loadJSONL(dataset);
const results = [];

for (const item of cases) {
  const session = await requestJSON('/agent/sessions', {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ title: `eval-${item.id}` })
  });
  const output = await streamAgentAnswer(token, session.session_id, item.query, item.attachments ?? []);
  results.push({
    id: item.id,
    query: item.query,
    answer: output.answer,
    blocks: output.blocks,
    run_id: output.run_id,
    trace_id: output.trace_id
  });
}

const report = {
  type: 'agent_e2e',
  dataset,
  generated_at: new Date().toISOString(),
  results
};
const file = await writeJSONReport('agent_e2e', report);
console.log(file);
