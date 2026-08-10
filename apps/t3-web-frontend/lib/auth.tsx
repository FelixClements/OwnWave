'use client';

import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { trpc } from '@/lib/trpc/client';
import { setAuthToken, getAuthToken, User, api } from '@/lib/api';

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

  const loginMutation = trpc.login.useMutation();
  const registerMutation = trpc.register.useMutation();

  useEffect(() => {
    const token = localStorage.getItem('ownwave:token');
    if (token) {
      setAuthToken(token);
      loadUser().finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, []);

  const loadUser = async () => {
    const me = await api.me();
    if (me) {
      setUser(me);
    } else {
      setAuthToken(null);
      localStorage.removeItem('ownwave:token');
      setUser(null);
    }
  };

  const login = async (username: string, password: string) => {
    const data = await loginMutation.mutateAsync({ username, password });
    setAuthToken(data.token);
    localStorage.setItem('ownwave:token', data.token);
    await loadUser();
  };

  const register = async (username: string, password: string) => {
    const data = await registerMutation.mutateAsync({ username, password });
    setAuthToken(data.token);
    localStorage.setItem('ownwave:token', data.token);
    await loadUser();
  };

  const logout = async () => {
    try {
      await api.logout();
    } finally {
      setAuthToken(null);
      setUser(null);
      localStorage.removeItem('ownwave:token');
    }
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
