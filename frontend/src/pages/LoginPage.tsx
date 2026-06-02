import { FormEvent, useState } from 'react';
import { login, register } from '../api/auth';
import type { AuthSession } from '../types/auth';

type LoginPageProps = {
  onLogin: (session: AuthSession) => void;
};

const demoAccounts = [
  { role: '用户端', username: 'user', password: 'user123456' },
  { role: '商家端', username: 'merchant', password: 'merchant123456' },
  { role: '管理员端', username: 'admin', password: 'admin123456' }
];

export function LoginPage({ onLogin }: LoginPageProps) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
	const [username, setUsername] = useState('user');
	const [password, setPassword] = useState('user123456');
  const [displayName, setDisplayName] = useState('');
	const [error, setError] = useState('');
	const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError('');
    setSubmitting(true);
	try {
		const session =
        mode === 'login'
          ? await login({ username, password })
          : await register({ username, password, display_name: displayName || username });
		onLogin(session);
	} catch {
		setError(mode === 'login' ? '账号或密码错误' : '注册失败，请检查账号是否已存在');
	} finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel">
        <div className="brand login-brand">
          <span className="brand-mark">AI</span>
          <div>
            <strong>小猪小狗导购</strong>
            <small>三端演示入口</small>
          </div>
        </div>
        <div className="auth-tabs">
          <button type="button" className={mode === 'login' ? 'active' : ''} onClick={() => setMode('login')}>
            登录
          </button>
          <button
            type="button"
            className={mode === 'register' ? 'active' : ''}
            onClick={() => {
              setMode('register');
              setUsername('');
              setPassword('');
              setDisplayName('');
              setError('');
            }}
          >
            注册
          </button>
        </div>
        <form className="login-form" onSubmit={submit}>
          <label>
            账号
            <input value={username} onChange={(event) => setUsername(event.target.value)} />
          </label>
          {mode === 'register' ? (
            <label>
              昵称
              <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
            </label>
          ) : null}
          <label>
            密码
            <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} />
          </label>
          {error ? <p className="form-error">{error}</p> : null}
          <button className="button" disabled={submitting}>
            {submitting ? (mode === 'login' ? '登录中' : '注册中') : mode === 'login' ? '登录' : '注册并进入'}
          </button>
        </form>
        {mode === 'login' ? <div className="demo-account-grid">
          {demoAccounts.map((item) => (
            <button
              className="demo-account"
              key={item.username}
              onClick={() => {
                setUsername(item.username);
                setPassword(item.password);
              }}
            >
              <strong>{item.role}</strong>
              <span>{item.username}</span>
            </button>
          ))}
        </div> : null}
      </section>
    </main>
  );
}
