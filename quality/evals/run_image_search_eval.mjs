import { authHeaders, loadJSONL, login, requestJSON, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/image_search_cases.jsonl';
const username = process.env.EVAL_ADMIN_USERNAME ?? 'admin';
const password = process.env.EVAL_ADMIN_PASSWORD ?? 'admin123456';

const token = await login(username, password);
const cases = await loadJSONL(dataset);
const results = [];

for (const item of cases) {
  const startedAt = Date.now();
  const response = await requestJSON('/eval/image-search', {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({
      file_id: item.file_id,
      object_key: item.object_key,
      image_url: item.image_url,
      top_k: item.top_k ?? 5
    })
  });
  const latencyMS = Date.now() - startedAt;
  const expectedStatus = item.expected_status ?? 'no_match';
  const passed = response.relevance_status === expectedStatus;
  results.push({
    id: item.id,
    case_type: item.case_type ?? 'unknown',
    expected_status: expectedStatus,
    actual_status: response.relevance_status,
    match_status: response.match_status,
    durations: response.durations,
    latency_ms: latencyMS,
    passed
  });
}

const evaluated = results.length;
const hits = results.filter((item) => item.passed).length;
const report = {
  type: 'image_search_eval',
  dataset,
  generated_at: new Date().toISOString(),
  total: results.length,
  evaluated,
  hits,
  pass_rate: evaluated ? hits / evaluated : 0,
  latency_ms: latencyStats(results.map((item) => item.latency_ms)),
  image_total_duration_ms_p95: percentile(results.map((item) => item.durations?.total_ms ?? item.latency_ms), 0.95),
  image_download_duration_ms_p95: percentile(results.map((item) => item.durations?.download_ms ?? 0), 0.95),
  image_embedding_duration_ms_p95: percentile(results.map((item) => item.durations?.embedding_ms ?? 0), 0.95),
  image_vector_search_duration_ms_p95: percentile(results.map((item) => item.durations?.vector_search_ms ?? 0), 0.95),
  image_rerank_duration_ms_p95: percentile(results.map((item) => item.durations?.rerank_ms ?? 0), 0.95),
  by_case_type: groupStats(results),
  results
};

const file = await writeJSONReport('image_search_eval', report);
console.log(file);

function latencyStats(values) {
  return {
    avg: values.length ? values.reduce((sum, value) => sum + value, 0) / values.length : 0,
    p50: percentile(values, 0.5),
    p95: percentile(values, 0.95)
  };
}

function percentile(values, p) {
  const sorted = values.filter((value) => Number.isFinite(value)).sort((a, b) => a - b);
  if (sorted.length === 0) return 0;
  const index = Math.min(sorted.length - 1, Math.ceil(sorted.length * p) - 1);
  return sorted[index];
}

function groupStats(items) {
  const groups = new Map();
  for (const item of items) {
    const key = item.case_type ?? 'unknown';
    const group = groups.get(key) ?? { total: 0, hits: 0, latency_values: [] };
    group.total += 1;
    group.hits += item.passed ? 1 : 0;
    group.latency_values.push(item.latency_ms);
    groups.set(key, group);
  }
  return Object.fromEntries(
    Array.from(groups.entries()).map(([key, group]) => [
      key,
      {
        total: group.total,
        hits: group.hits,
        pass_rate: group.total ? group.hits / group.total : 0,
        latency_ms: latencyStats(group.latency_values)
      }
    ])
  );
}
