import { authHeaders, loadJSONL, login, requestJSON, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/intent_cases.jsonl';
const username = process.env.EVAL_ADMIN_USERNAME ?? 'admin';
const password = process.env.EVAL_ADMIN_PASSWORD ?? 'admin123456';

const token = await login(username, password);
const cases = await loadJSONL(dataset);
const results = [];

for (const item of cases) {
  const response = await requestJSON('/eval/intent', {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ query: item.query })
  });
  results.push({
    id: item.id,
    query: item.query,
    route: response.route,
    expected_route: item.expected_route,
    intent: response.intent,
    expected_intent: item.expected_intent,
    group: item.group ?? item.expected_intent,
    correct: response.intent === item.expected_intent && (!item.expected_route || response.route === item.expected_route)
  });
}

const correct = results.filter((item) => item.correct).length;
const byGroup = groupStats(results);
const report = {
  type: 'intent_classification',
  dataset,
  generated_at: new Date().toISOString(),
  total: results.length,
  correct,
  accuracy: results.length ? correct / results.length : 0,
  by_group: byGroup,
  results
};
const file = await writeJSONReport('intent_eval', report);
console.log(file);
if (correct !== results.length) process.exitCode = 1;

function groupStats(items) {
  const groups = new Map();
  for (const item of items) {
    const key = item.group ?? 'unknown';
    const group = groups.get(key) ?? { total: 0, correct: 0 };
    group.total += 1;
    group.correct += item.correct ? 1 : 0;
    groups.set(key, group);
  }
  return Object.fromEntries(
    Array.from(groups.entries()).map(([key, group]) => [
      key,
      {
        total: group.total,
        correct: group.correct,
        accuracy: group.total ? group.correct / group.total : null
      }
    ])
  );
}
