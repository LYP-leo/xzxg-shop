import { useState } from 'react';
import {
  AgentPrompt,
  AgentPromptPublishRecord,
  listAgentPromptsPage,
  publishAgentPrompt,
  saveAgentPromptDraft
} from '../../api/admin';
import { PageSizeSelect, PaginationBar, PanelMessage, formatDateTime, totalPages, useAsyncList, useNotice } from './AdminCommon';

export function AdminPromptsPage({ token }: { token: string }) {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [records, setRecords] = useState<AgentPromptPublishRecord[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [savingKey, setSavingKey] = useState('');
  const [publishingKey, setPublishingKey] = useState('');
  const notice = useNotice();
  const prompts = useAsyncList<AgentPrompt>(
    async () => {
      const data = await listAgentPromptsPage(token, page, pageSize);
      setRecords(data.publish_records ?? []);
      setDrafts((current) => ({
        ...current,
        ...Object.fromEntries(data.items.map((item) => [item.prompt_key, current[item.prompt_key] ?? item.content]))
      }));
      return data;
    },
    [token, page, pageSize]
  );
  const pages = totalPages(prompts.total, pageSize);

  async function savePrompt(prompt: AgentPrompt) {
    setSavingKey(prompt.prompt_key);
    notice.clear();
    try {
      await saveAgentPromptDraft(token, prompt.prompt_key, {
        title: prompt.title,
        description: prompt.description,
        content: drafts[prompt.prompt_key] ?? ''
      });
      notice.showNotice(`${prompt.prompt_key} 草稿已保存`);
      await prompts.reload();
    } catch (error) {
      notice.showError(error);
    } finally {
      setSavingKey('');
    }
  }

  async function publishPrompt(prompt: AgentPrompt) {
    setPublishingKey(prompt.prompt_key);
    notice.clear();
    try {
      await publishAgentPrompt(token, prompt.prompt_key);
      notice.showNotice(`${prompt.prompt_key} 已发布生效`);
      await prompts.reload();
    } catch (error) {
      notice.showError(error);
    } finally {
      setPublishingKey('');
    }
  }

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>Prompt 管理</h1>
          <p>编辑 Prompt 草稿，发布后写入数据库 active 版本；运行时只从数据库读取 Prompt。</p>
        </div>
        <button className="button button--ghost" disabled={prompts.loading} onClick={prompts.reload}>
          {prompts.loading ? '刷新中' : '刷新'}
        </button>
      </header>
      {notice.notice ? <p className="notice">{notice.notice}</p> : null}
      {notice.error ? <p className="form-error">{notice.error}</p> : null}
      <div className="admin-grid">
        <section className="panel prompt-panel">
          <div className="panel-title-row">
            <div>
              <h2>Agent Prompt</h2>
              <p>
                共 {prompts.total} 个 Prompt，第 {page} / {pages} 页
              </p>
            </div>
            <PageSizeSelect
              value={pageSize}
              onChange={(value) => {
                setPageSize(value);
                setPage(1);
              }}
            />
          </div>
          <div className="prompt-list">
            {prompts.items.map((prompt) => (
              <article className="prompt-editor" key={prompt.prompt_key}>
                <div className="prompt-editor__header">
                  <div>
                    <strong>{prompt.prompt_key}</strong>
                    <span className="tag">{prompt.status}</span>
                    <span className="tag">v{prompt.version}</span>
                  </div>
                  <div className="panel-actions-inline">
                    <button className="button button--ghost" disabled={savingKey === prompt.prompt_key} onClick={() => savePrompt(prompt)}>
                      {savingKey === prompt.prompt_key ? '保存中' : '保存草稿'}
                    </button>
                    <button className="button" disabled={publishingKey === prompt.prompt_key} onClick={() => publishPrompt(prompt)}>
                      {publishingKey === prompt.prompt_key ? '发布中' : '发布生效'}
                    </button>
                  </div>
                </div>
                <p>{prompt.description || prompt.title}</p>
                <textarea
                  className="prompt-editor__textarea"
                  value={drafts[prompt.prompt_key] ?? ''}
                  onChange={(event) => setDrafts((current) => ({ ...current, [prompt.prompt_key]: event.target.value }))}
                />
                <small>最近更新 {formatDateTime(prompt.updated_at)}</small>
              </article>
            ))}
          </div>
          <PanelMessage loading={prompts.loading} error={prompts.error} empty={!prompts.items.length ? '暂无 Prompt' : ''} />
          <PaginationBar
            loading={prompts.loading}
            page={page}
            totalPages={pages}
            visibleCount={prompts.items.length}
            total={prompts.total}
            onPrev={() => setPage((current) => Math.max(1, current - 1))}
            onNext={() => setPage((current) => Math.min(pages, current + 1))}
          />
        </section>
        <section className="panel">
          <div className="panel-title-row">
            <div>
              <h2>最近发布</h2>
              <p>展示最近 20 次 Prompt 发布记录。</p>
            </div>
          </div>
          <div className="table-scroll">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Prompt</th>
                  <th>版本</th>
                  <th>生效位置</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                {records.map((record) => (
                  <tr key={record.record_id}>
                    <td>{record.prompt_key}</td>
                    <td>v{record.version}</td>
                    <td>{record.publish_target || 'database'}</td>
                    <td>{formatDateTime(record.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {!records.length ? <p className="empty-text">暂无发布记录</p> : null}
        </section>
      </div>
    </section>
  );
}
