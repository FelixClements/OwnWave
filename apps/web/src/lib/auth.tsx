import { createContext, useContext, useEffect, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError, api, call, setUnauthorizedHandler } from '@/api/client';
import type { User } from '@/api/types';
import { keys } from './queries';

type AuthValue = {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string, inviteToken?: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const me = useQuery({
    queryKey: keys.me,
    queryFn: async () => {
      try {
        return await call(api.GET('/api/me'));
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null;
        throw e;
      }
    },
    staleTime: Infinity,
  });

  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (qc.getQueryData(keys.me)) qc.setQueryData(keys.me, null);
    });
  }, [qc]);

  const refresh = async () => {
    qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' });
    await qc.invalidateQueries({ queryKey: keys.me });
    await qc.invalidateQueries({ queryKey: keys.setup });
  };

  const value: AuthValue = {
    user: me.data ?? null,
    loading: me.isLoading,
    login: async (username, password) => {
      await call(api.POST('/api/login', { body: { username, password } }));
      await refresh();
    },
    register: async (username, password, inviteToken) => {
      await call(api.POST('/api/register', { body: { username, password, invite_token: inviteToken ?? '' } }));
      await refresh();
    },
    logout: async () => {
      await call(api.POST('/api/logout')).catch(() => {});
      qc.clear();
      qc.setQueryData(keys.me, null);
    },
  };
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthValue {
  const v = useContext(AuthContext);
  if (!v) throw new Error('useAuth outside AuthProvider');
  return v;
}
