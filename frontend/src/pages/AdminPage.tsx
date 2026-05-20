import { useEffect, useState } from 'react';
import { DocumentItem, EvalRun, listAccounts, listDocuments, listEvalRuns } from '../api/admin';
import type { Account } from '../types/auth';

type AdminPageProps = {
  token: string;
};

export function AdminPage({ token }: AdminPageProps) {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [evalRuns, setEvalRuns] = useState<EvalRun[]>([]);

  useEffect(() => {
    listAccounts(token).then(setAccounts);
    listDocuments(token).then(setDocuments);
    listEvalRuns().then(setEvalRuns);
  }, [token]);

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>管理与评测</h1>
          <p>用于演示文档状态、知识库构建和评测闭环。</p>
        </div>
      </header>
      <div className="admin-grid">
        <section className="panel">
          <h2>账号</h2>
          <table className="data-table">
            <thead>
              <tr>
                <th>账号</th>
                <th>角色</th>
                <th>名称</th>
              </tr>
            </thead>
            <tbody>
              {accounts.map((account) => (
                <tr key={account.account_id}>
                  <td>{account.username}</td>
                  <td>{account.role}</td>
                  <td>{account.display_name}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
        <section className="panel">
          <h2>文档</h2>
          <table className="data-table">
            <thead>
              <tr>
                <th>标题</th>
                <th>类型</th>
                <th>状态</th>
                <th>Chunk</th>
              </tr>
            </thead>
            <tbody>
              {documents.map((document) => (
                <tr key={document.documentId}>
                  <td>{document.title}</td>
                  <td>{document.docType}</td>
                  <td>{document.status}</td>
                  <td>{document.chunkCount}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
        <section className="panel">
          <h2>评测</h2>
          <table className="data-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>状态</th>
                <th>通过率</th>
              </tr>
            </thead>
            <tbody>
              {evalRuns.map((run) => (
                <tr key={run.evalRunId}>
                  <td>{run.name}</td>
                  <td>{run.status}</td>
                  <td>{run.passRate}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </div>
    </section>
  );
}
