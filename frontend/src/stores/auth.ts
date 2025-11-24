import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { AuthState, User } from '@/types'
import { setAuthToken, clearAuthToken } from '@/lib/api'

interface AuthStore extends AuthState {
  login: (token: string, user: User) => void
  logout: () => void
  clearAuth: () => void
  setUser: (user: User) => void
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      isAuthenticated: false,

      login: (token: string, user: User) => {
        setAuthToken(token)
        set({ token, user, isAuthenticated: true })
      },

      logout: () => {
        clearAuthToken()
        set({ user: null, token: null, isAuthenticated: false })
      },

      clearAuth: () => {
        clearAuthToken()
        set({ user: null, token: null, isAuthenticated: false })
      },

      setUser: (user: User) => {
        set({ user })
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
