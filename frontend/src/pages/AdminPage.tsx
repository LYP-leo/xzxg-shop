import { AdminDebugPage } from './admin/AdminDebugPage';
import { AdminEvalPage } from './admin/AdminEvalPage';
import { AdminPlatformPage } from './admin/AdminPlatformPage';
import { AdminPromptsPage } from './admin/AdminPromptsPage';
import { AdminRiskPage } from './admin/AdminRiskPage';

type AdminPageProps = {
  token: string;
  view?: 'platform' | 'debug' | 'eval' | 'prompts' | 'risk';
};

export function AdminPage({ token, view = 'platform' }: AdminPageProps) {
  if (view === 'prompts') return <AdminPromptsPage token={token} />;
  if (view === 'debug') return <AdminDebugPage token={token} />;
  if (view === 'eval') return <AdminEvalPage token={token} />;
  if (view === 'risk') return <AdminRiskPage token={token} />;
  return <AdminPlatformPage token={token} />;
}
