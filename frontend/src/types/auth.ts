export type AccountRole = 'user' | 'merchant' | 'admin';

export type Account = {
  account_id: string;
  username: string;
  display_name: string;
  role: AccountRole;
  merchant_id?: string;
  created_at: string;
};

export type AuthSession = {
  token: string;
  account: Account;
};
