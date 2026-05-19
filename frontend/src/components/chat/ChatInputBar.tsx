import { useState } from 'react';

type Props = {
  disabled: boolean;
  onSend: (content: string) => void;
  onCancel?: () => void;
};

export function ChatInputBar({ disabled, onSend, onCancel }: Props) {
  const [content, setContent] = useState('');

  function submit() {
    const trimmed = content.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setContent('');
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
        {disabled ? (
          <button className="button button--ghost" onClick={onCancel}>
            停止
          </button>
        ) : null}
        <button className="button" disabled={disabled || !content.trim()} onClick={submit}>
          发送
        </button>
      </div>
    </div>
  );
}
