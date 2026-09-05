import test from 'node:test';
import assert from 'node:assert/strict';
import { consumeAgentStream } from './stream_state.mjs';
import { evaluateShoppingCase } from './shopping_assertions.mjs';

const frame = value => `event: ${value.type}\r\ndata: ${JSON.stringify(value)}\r\n\r\n`;
const start = { type: 'message_start', run_id: 'run1' };
const end = { type: 'message_end', run_id: 'run1' };
function body(text, chunkSize = 1) {
  const bytes = new TextEncoder().encode(text);
  return new ReadableStream({ start(controller) {
    for (let i = 0; i < bytes.length; i += chunkSize) controller.enqueue(bytes.slice(i, i + chunkSize));
    controller.close();
  } });
}

test('content_delta cards and fragmented Unicode are part of the evaluated display', async () => {
  const output = await consumeAgentStream(body(frame(start) + frame({ type: 'text_delta', delta: '你好😀' }) + frame({ type: 'content_delta', part: { type: 'product_card', product: { productId: 'p_1' } } }) + frame(end)));
  assert.equal(output.answer, '你好😀');
  assert.equal(output.blocks[0].product.productId, 'p_1');
  assert.equal(evaluateShoppingCase({ expected_no_product_refs: true }, output).passed, false);
  assert.equal(evaluateShoppingCase({ expected_product_ids: ['p_1'] }, output).passed, true);
});

test('multiline SSE data and comments follow SSE framing', async () => {
  const output = await consumeAgentStream(body(': ping\r\n\r\n' + frame(start) + 'event: text_delta\ndata: {"type":"text_delta",\ndata: "delta":"hello"}\n\n' + frame(end), 7));
  assert.equal(output.answer, 'hello');
});

test('EOF, server error, mixed runs and malformed events cannot pass', async () => {
  for (const text of [
    frame(start),
    frame(start) + frame({ type: 'error', code: 'tool_failed' }) + frame(end),
    frame(start) + frame({ type: 'text_delta', run_id: 'other', delta: 'oops' }) + frame(end),
    frame(start) + 'data: {broken}\n\n' + frame(end),
    frame(start) + frame(end).trimEnd(),
    frame(start) + frame(end) + frame({ type: 'text_delta', delta: 'late' })
  ]) await assert.rejects(consumeAgentStream(body(text)));
});

test('extra items, wrong SKU or wrong quantity fail exact business assertions', () => {
  const item = { product_id: 'p_1', sku_id: 'black_256', quantity: 2 };
  for (const actual of [[{ ...item, quantity: 1 }], [{ ...item, sku_id: 'white_256' }], [item, { ...item, product_id: 'p_2' }]]) {
    assert.equal(evaluateShoppingCase({ expected_cart_items: [item] }, { complete: true, actual_cart: { items: actual } }).passed, false);
  }
  assert.equal(evaluateShoppingCase({ expected_cart_items: [item] }, { complete: true, actual_cart: { items: [item] } }).passed, true);
  assert.equal(evaluateShoppingCase({}, { complete: true }).evaluated, false);
});
