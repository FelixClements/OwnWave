'use client';

import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { trpc } from '@/lib/trpc/client';
import { setAuthToken, getAuthToken, User } from '@/lib/api';

type AuthContextValue = {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const me = trpc.me.useQuery(undefined, { enabled: false });
  const loginMutation = trpc.login.useMutation({
    onSuccess: (data) => {
      setAuthToken(data.token);
      setUser(data.user);
      localStorage.setItem('ownwave:token', data.token);
    },
  });
  const registerMutation = trpc.register.useMutation({
    onSuccess: (data) => {
      setAuthToken(data.token);
      setUser(data.user);
      localStorage.setItem('ownwave:token', data.token);
    },
  });
  const logoutMutation = trpc.logout.useMutation({
    onSuccess: () => {
      setAuthToken(null);
      setUser(null);
      localStorage.removeItem('ownwave:token');
    },
  });

  useEffect(() => {
    const token = localStorage.getItem('ownwave:token');
    if (token) {
      setAuthToken(token);
      me.refetch();
    } else {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (me.data) {
      setUser(me.data);
    }
    if (!me.isLoading) {
      setLoading(false);
    }
  }, [me.data, me.isLoading]);

  const login = async (username: string, password: string) => {
    await loginMutation.mutateAsync({ username, password });
  };

  const register = async (username: string, password: string) => {
    await registerMutation.mutateAsync({ username, password });
  };

  const logout = async () => {
    await logoutMutation.mutateAsync();
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider');
  return ctx;
}
