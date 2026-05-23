import { authHeaders, loadJSONL, login, requestJSON, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/rag_recall_cases.jsonl';
const username = process.env.EVAL_ADMIN_USERNAME ?? 'admin';
const password = process.env.EVAL_ADMIN_PASSWORD ?? 'admin123456';

const token = await login(username, password);
const cases = await loadJSONL(dataset);
const results = [];

for (const item of cases) {
  const response = await requestJSON('/eval/rag', {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ keyword: item.keyword, top_k: item.top_k ?? 5 })
  });
  const recalledIDs = response.items.map((citation) => citation.chunkId);
  const expected = item.expected_recall ?? [];
  results.push({
    id: item.id,
    keyword: item.keyword,
    recalled: response.items,
    expected_recall: expected,
    hit: expected.length === 0 ? null : expected.some((id) => recalledIDs.includes(id))
  });
}

const evaluated = results.filter((item) => item.hit !== null);
const hits = evaluated.filter((item) => item.hit).length;
const report = {
  type: 'rag_recall',
  dataset,
  generated_at: new Date().toISOString(),
  total: results.length,
  evaluated: evaluated.length,
  hits,
  recall_case_hit_rate: evaluated.length ? hits / evaluated.length : null,
  results
};
const file = await writeJSONReport('rag_recall_eval', report);
console.log(file);
if (evaluated.length > 0 && hits !== evaluated.length) process.exitCode = 1;
