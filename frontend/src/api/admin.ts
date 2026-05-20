export type DocumentItem = {
  documentId: string;
  title: string;
  docType: string;
  status: 'uploaded' | 'parsing' | 'indexing' | 'indexed' | 'failed';
  chunkCount: number;
};

export type EvalRun = {
  evalRunId: string;
  name: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  passRate: string;
};

export async function listDocuments(): Promise<DocumentItem[]> {
  return [
    {
      documentId: 'doc_001',
      title: '手机商品详情',
      docType: 'product_detail',
      status: 'indexed',
      chunkCount: 18
    },
    {
      documentId: 'doc_002',
      title: '618 活动规则',
      docType: 'promotion',
      status: 'indexed',
      chunkCount: 9
    }
  ];
}

export async function listEvalRuns(): Promise<EvalRun[]> {
  return [
    {
      evalRunId: 'eval_001',
      name: '手机推荐核心集',
      status: 'completed',
      passRate: '86%'
    }
  ];
}
