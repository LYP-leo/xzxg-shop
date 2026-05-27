import { useState } from 'react';
import { uploadFile } from '../../api/files';
import type { Attachment } from '../../types/agent';

type Props = {
  disabled: boolean;
  onSend: (content: string, attachments?: Attachment[]) => void;
  onCancel?: () => void;
};

export function ChatInputBar({ disabled, onSend, onCancel }: Props) {
  const [content, setContent] = useState('');
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [uploading, setUploading] = useState(false);

  function submit() {
    const trimmed = content.trim();
    if ((!trimmed && attachments.length === 0) || disabled || uploading) return;
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
            disabled={disabled || uploading}
            type="file"
            onChange={async (event) => {
              const file = event.target.files?.[0];
              if (!file) return;
              setUploading(true);
              try {
                const uploaded = await uploadFile(file);
                setAttachments([
                  {
                    attachmentId: uploaded.file_id,
                    type: 'image',
                    url: uploaded.url,
                    objectKey: uploaded.object_key,
                    name: file.name
                  }
                ]);
              } finally {
                setUploading(false);
              }
              event.target.value = '';
            }}
          />
        </label>
        {disabled ? (
          <button className="button button--ghost" onClick={onCancel}>
            停止
          </button>
        ) : null}
        <button className="button" disabled={disabled || uploading || (!content.trim() && attachments.length === 0)} onClick={submit}>
          发送
        </button>
      </div>
      {uploading ? <div className="attachment-preview">图片上传中...</div> : null}
      {attachments.length ? <div className="attachment-preview">已上传图片：{attachments[0].name ?? 'photo'}</div> : null}
    </div>
  );
}
