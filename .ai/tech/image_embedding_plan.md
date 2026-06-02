# 图片 Embedding 升级方案

更新时间：2026-06-02

## 当前状态

- 当前图片向量链路已经接入 Milvus，collection 为 `product_image_vectors`。
- 当前向量生成方式是后端本地 `imagevector.FromImage`：64 维颜色直方图。
- 当前支持格式：JPEG、PNG、GIF、WebP。
- 当前优点：不依赖外部模型，速度快，成本为 0。
- 当前缺点：只能表达颜色分布，不能理解商品语义、物体类别、风格、材质、品牌和场景，因此会出现颜色相近但语义无关的召回。

## 推荐升级目标

图片搜索需要从“颜色相似”升级为“语义相似”。推荐接百炼多模态 Embedding：

1. 低成本优先：`tongyi-embedding-vision-flash-2026-03-06`
   - 支持 64、128、256、512、768 维。
   - 适合当前项目先低成本验证图搜图、图文融合召回。
   - 可以先使用 256 或 512 维，效果和成本比较均衡。

2. 效果优先：`qwen3-vl-embedding`
   - 支持 256、512、768、1024、1536、2048、2560 维。
   - 适合后续做图搜图、文搜图、商品主图+标题融合向量。
   - 成本更高，默认维度更大，Milvus 存储和检索开销也更高。

注意：百炼多模态 Embedding 不走 OpenAI compatible `/embeddings` 接口，需要调用 DashScope 原生多模态 Embedding API。

## 本项目推荐选型

按“效果优先，但用户单次请求 RT 可控”的原则，推荐默认选：

```text
image_embedding.provider=dashscope
image_embedding.model=qwen3-vl-embedding
image_embedding.dimension=512
image_embedding.mode=image
milvus.collection.product_images=product_image_vectors_v2
```

原因：

- `qwen3-vl-embedding` 是效果优先方案，适合图搜图、文搜图、商品图文混合检索。
- 不使用默认 2560 维，先用 512 维平衡效果、Milvus 存储、检索速度和网络传输体积。
- 256 维适合更强性能约束，但细粒度商品相似度会更容易损失。
- 768/1024 维可作为二阶段评测升级项，如果 Top1/Top3 明显不足再提高维度。
- 请求侧只对“用户上传/引用的查询图片”在线调用一次模型；商品库图片向量必须离线批量构建，不能在用户请求里现算商品侧向量。

RT 控制目标：

```text
图片下载/读取 P95 <= 300ms
DashScope 图片 embedding P95 <= 1500ms
Milvus 检索 P95 <= 100ms
后端整体图片搜索 P95 <= 2200ms
```

降级策略：

- DashScope 调用失败或超时时，直接返回 `image_embedding_unavailable`，不要拿 64 维本地向量去查 512 维 v2 collection。
- 保留旧的 `product_image_vectors` 64 维 collection 作为独立 fallback，只有显式切换 `image_embedding.provider=local_histogram` 时才使用。
- 对用户上传图片计算 SHA-256，短期缓存图片 embedding，避免同一图片重复请求模型。

## 配置建议

新增 Nacos 配置：

```text
image_embedding.provider=dashscope|local_histogram
image_embedding.base_url=https://dashscope.aliyuncs.com
image_embedding.api_key=<复用 ai.api_key 或单独配置>
image_embedding.model=tongyi-embedding-vision-flash-2026-03-06
image_embedding.dimension=256
image_embedding.mode=image|fusion
milvus.collection.product_images=product_image_vectors_v2
```

说明：

- `local_histogram` 作为兜底方案，外部模型不可用时仍可完成基础图搜。
- 如果从 64 维升级到 256/512 维，必须新建 Milvus collection，不能复用当前 `product_image_vectors`，因为 Milvus collection 维度固定。
- 推荐先使用 `product_image_vectors_v2`，回滚时还能切回旧 collection。

## 后端改造点

1. 增加 `imagevector.Embedder` 接口：

```go
type Embedder interface {
    EmbedImage(ctx context.Context, image ImageInput) ([]float32, error)
    Dimension() int
}
```

2. 保留本地实现：

```text
LocalHistogramEmbedder -> 64 维
```

3. 新增 DashScope 多模态实现：

```text
DashScopeImageEmbedder -> 调用 /api/v1/services/embeddings/multimodal-embedding/multimodal-embedding
```

4. 数据入库：

- 商品图片向量构建时优先用 DashScope。
- 请求图片搜索时也用同一个 DashScope embedder。
- DashScope 调用失败时，按配置决定是否降级到本地 64 维。注意：如果 Milvus collection 是 256/512 维，不能用 64 维向量降级搜索，只能返回 `image_embedding_unavailable` 或切旧 collection。

5. 测评：

- 图片搜索测评增加维度：命中率、Top1/Top3、平均耗时、P95 耗时、模型失败率、降级率。
- 单独记录 `download_ms`、`embedding_ms`、`vector_search_ms`、`rerank_ms`。

## 推荐落地顺序

1. 当前版本：补齐 WebP 支持，保持 64 维本地向量可用。
2. 下一版：增加 DashScope 多模态图片 Embedder 和 Nacos 配置。
3. 新建 `product_image_vectors_v2`，批量重建商品图片向量。
4. 管理员页面增加图片向量版本、模型、维度、row_count、命中率、耗时趋势。
5. 评测通过后切 `milvus.collection.product_images` 到 v2。
