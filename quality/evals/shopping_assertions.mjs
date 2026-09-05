export function productIDsFromBlocks(blocks) {
  const ids = [];
  for (const block of blocks) {
    if (Array.isArray(block.product_ids)) ids.push(...block.product_ids);
    const id = block.product?.productId ?? block.product?.product_id;
    if (id) ids.push(id);
  }
  return [...new Set(ids)];
}

export function sameSet(actual, expected) {
  const a = [...new Set(actual)].sort();
  const b = [...new Set(expected)].sort();
  return JSON.stringify(a) === JSON.stringify(b);
}

export function cartLines(cart) {
  return (cart?.items ?? []).map(item => ({
    product_id: item.productId ?? item.product_id,
    sku_id: item.skuId ?? item.sku_id ?? '',
    quantity: item.quantity,
    selected: item.selected ?? true
  })).sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
}

export function evaluateShoppingCase(item, output) {
  const checks = [];
  const answer = output.answer ?? '';
  const productIDs = productIDsFromBlocks(output.blocks ?? []);
  // Transport success is necessary, but not a business assertion. A case with
  // no expected outcome remains uncovered even if the stream completed.
  if (item.expected_no_product_refs) {
    const textIDs = [...new Set(Array.from(answer.matchAll(/\bp_[A-Za-z0-9_]+\b/g), m => m[0]))];
    checks.push({ name: 'no_product_refs', passed: !productIDs.length && !textIDs.length, product_ids: productIDs, text_product_ids: textIDs });
  }
  if (Array.isArray(item.expected_product_ids)) checks.push({ name: 'exact_displayed_products', passed: sameSet(productIDs, item.expected_product_ids), actual: productIDs, expected: item.expected_product_ids });
  if (item.forbidden_terms?.length) {
    const found = item.forbidden_terms.filter(term => answer.includes(term));
    checks.push({ name: 'forbidden_terms', passed: !found.length, found });
  }
  if (item.expected_ack_terms?.length) checks.push({ name: 'expected_ack_terms', passed: item.expected_ack_terms.some(term => answer.includes(term)) });
  if (Array.isArray(item.expected_cart_items)) {
    const actual = cartLines(output.actual_cart);
    const expected = cartLines({ items: item.expected_cart_items });
    checks.push({ name: 'exact_cart_lines', passed: !!output.actual_cart && JSON.stringify(actual) === JSON.stringify(expected), actual, expected });
  }
  if (item.expect_cart_unchanged) {
    checks.push({ name: 'cart_unchanged', passed: !!output.cart_before && !!output.actual_cart && JSON.stringify(cartLines(output.cart_before)) === JSON.stringify(cartLines(output.actual_cart)) });
  }
  const evaluated = checks.length > 0;
  checks.push({ name: 'complete_stream', passed: output.complete === true && !output.errors?.length && !output.transport_error });
  return { evaluated, passed: evaluated && checks.every(check => check.passed), checks };
}
