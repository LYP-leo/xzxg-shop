import { authHeaders, loadJSONL, login, requestJSON, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/rag_recall_cases.jsonl';
const username = process.env.EVAL_ADMIN_USERNAME ?? 'admin';
const password = process.env.EVAL_ADMIN_PASSWORD ?? 'admin123456';
const minHitRate = Number(process.env.RAG_RECALL_MIN_HIT_RATE ?? 0.8);
const delayMS = Number(process.env.RAG_EVAL_DELAY_MS ?? 250);

const token = await login(username, password);
const cases = await loadJSONL(dataset);
const results = [];

for (const item of cases) {
  const query = item.query ?? item.keyword;
  const response = await requestRAGWithRetry(token, query, item.top_k ?? 10);
  const recalledIDs = response.items.map((citation) => citation.chunkId);
  const recalledSources = response.items.map((citation) => citation.source).filter(Boolean);
  const expectedChunks = item.expected_chunk_ids ?? item.expected_recall ?? [];
  const expectedProducts = item.expected_product_ids ?? item.expected_sources ?? [];
  const chunkRank = firstRank(recalledIDs, expectedChunks);
  const productRank = firstRank(recalledSources, expectedProducts);
  const bestRank = minPositive(chunkRank, productRank);
  results.push({
    id: item.id,
    query,
    query_type: item.query_type ?? 'unknown',
    recalled: response.items,
    expected_chunk_ids: expectedChunks,
    expected_product_ids: expectedProducts,
    chunk_rank: chunkRank,
    product_rank: productRank,
    best_rank: bestRank,
    reciprocal_rank: bestRank > 0 ? 1 / bestRank : 0,
    hit: expectedChunks.length === 0 && expectedProducts.length === 0 ? null : bestRank > 0
  });
  if (delayMS > 0) {
    await sleep(delayMS);
  }
}

const evaluated = results.filter((item) => item.hit !== null);
const hits = evaluated.filter((item) => item.hit).length;
const mrr = evaluated.length ? evaluated.reduce((total, item) => total + item.reciprocal_rank, 0) / evaluated.length : null;
const hitRate = evaluated.length ? hits / evaluated.length : null;
const report = {
  type: 'rag_retriever_eval',
  dataset,
  generated_at: new Date().toISOString(),
  total: results.length,
  evaluated: evaluated.length,
  hits,
  hit_rate_at_k: hitRate,
  recall_case_hit_rate: hitRate,
  mrr,
  min_hit_rate: minHitRate,
  by_query_type: groupStats(evaluated),
  results
};
const file = await writeJSONReport('rag_recall_eval', report);
console.log(file);
if (hitRate !== null && hitRate < minHitRate) process.exitCode = 1;

function firstRank(actual, expected) {
  if (!Array.isArray(expected) || expected.length === 0) return 0;
  for (let index = 0; index < actual.length; index += 1) {
    if (expected.includes(actual[index])) return index + 1;
  }
  return 0;
}

function minPositive(...values) {
  const positives = values.filter((value) => value > 0);
  return positives.length ? Math.min(...positives) : 0;
}

function groupStats(items) {
  const groups = new Map();
  for (const item of items) {
    const key = item.query_type ?? 'unknown';
    const group = groups.get(key) ?? { total: 0, hits: 0, reciprocal_rank_sum: 0 };
    group.total += 1;
    group.hits += item.hit ? 1 : 0;
    group.reciprocal_rank_sum += item.reciprocal_rank ?? 0;
    groups.set(key, group);
  }
  return Object.fromEntries(
    Array.from(groups.entries()).map(([key, group]) => [
      key,
      {
        total: group.total,
        hits: group.hits,
        hit_rate_at_k: group.total ? group.hits / group.total : null,
        mrr: group.total ? group.reciprocal_rank_sum / group.total : null
      }
    ])
  );
}

async function requestRAGWithRetry(token, query, topK) {
  let lastError;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    try {
      return await requestJSON('/eval/rag', {
        method: 'POST',
        headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, top_k: topK })
      });
    } catch (error) {
      lastError = error;
      const message = error instanceof Error ? error.message : String(error);
      if (!message.includes('429')) throw error;
      await sleep(1000 * (attempt + 1));
    }
  }
  throw lastError;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
