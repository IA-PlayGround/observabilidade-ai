import { createContext, useContext, useState, useCallback } from 'react';

interface AuthContextType {
  token: string | null;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType>({
  token: null,
  login: async () => false,
  logout: () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(
    localStorage.getItem('spectrum_token')
  );

  const login = useCallback(async (username: string, password: string) => {
    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      if (!res.ok) return false;
      const data = await res.json();
      setToken(data.token);
      localStorage.setItem('spectrum_token', data.token);
      return true;
    } catch {
      return false;
    }
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    localStorage.removeItem('spectrum_token');
  }, []);

  return (
    <AuthContext.Provider value={{ token, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}

export async function apiFetch(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem('spectrum_token');
  const res = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });
  if (res.status === 401) {
    localStorage.removeItem('spectrum_token');
    window.location.reload();
    throw new Error('Unauthorized');
  }
  return res;
}
