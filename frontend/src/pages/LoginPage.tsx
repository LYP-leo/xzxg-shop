import { FormEvent, useState } from 'react';
import { login } from '../api/auth';
import type { AuthSession } from '../types/auth';

type LoginPageProps = {
  onLogin: (session: AuthSession) => void;
};

const demoAccounts = [
  { role: '商家端', username: 'merchant', password: 'merchant123456' },
  { role: '管理员端', username: 'admin', password: 'admin123456' }
];

export function LoginPage({ onLogin }: LoginPageProps) {
	const [username, setUsername] = useState('admin');
	const [password, setPassword] = useState('admin123456');
	const [error, setError] = useState('');
	const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError('');
    setSubmitting(true);
	try {
		const session = await login({ username, password });
		onLogin(session);
	} catch {
		setError('账号或密码错误');
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
            <small>后台管理入口</small>
          </div>
        </div>
        <form className="login-form" onSubmit={submit}>
          <label>
            账号
            <input value={username} onChange={(event) => setUsername(event.target.value)} />
          </label>
          <label>
            密码
            <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} />
          </label>
          {error ? <p className="form-error">{error}</p> : null}
          <button className="button" disabled={submitting}>
            {submitting ? '登录中' : '登录'}
          </button>
        </form>
        <div className="demo-account-grid">
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
        </div>
      </section>
    </main>
  );
}
