import { useState } from 'react';
import type { Attachment } from '../../types/agent';

type Props = {
  disabled: boolean;
  onSend: (content: string, attachments?: Attachment[]) => void;
  onCancel?: () => void;
};

export function ChatInputBar({ disabled, onSend, onCancel }: Props) {
  const [content, setContent] = useState('');
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  function submit() {
    const trimmed = content.trim();
    if ((!trimmed && attachments.length === 0) || disabled) return;
    onSend(trimmed || '拍照找货', attachments);
    setContent('');
    setAttachments([]);
  }

  return (
    <div className="chat-input">
      <textarea
        value={content}
        disabled={disabled}
        placeholder="描述预算、使用场景、偏好，或问某个商品是否适合你"
        onChange={(event) => setContent(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            submit();
          }
        }}
      />
      <div className="chat-input__actions">
        <label className="button button--ghost upload-button">
          拍照
          <input
            accept="image/*"
            capture="environment"
            disabled={disabled}
            type="file"
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (!file) return;
              setAttachments([
                {
                  attachmentId: `img_${Date.now()}`,
                  type: 'image',
                  url: URL.createObjectURL(file),
                  name: file.name
                }
              ]);
              event.target.value = '';
            }}
          />
        </label>
        {disabled ? (
          <button className="button button--ghost" onClick={onCancel}>
            停止
          </button>
        ) : null}
        <button className="button" disabled={disabled || (!content.trim() && attachments.length === 0)} onClick={submit}>
          发送
        </button>
      </div>
      {attachments.length ? <div className="attachment-preview">已选择图片：{attachments[0].name ?? 'photo'}</div> : null}
    </div>
  );
}
