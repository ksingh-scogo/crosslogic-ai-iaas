import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { Layout } from '@/components/layout/Layout'
import { useAuthStore } from '@/stores/auth'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: () => {
    const { isAuthenticated } = useAuthStore.getState()
    if (!isAuthenticated) {
      throw redirect({
        to: '/login',
      })
    }
  },
  component: AuthenticatedLayout,
})

function AuthenticatedLayout() {
  return (
    <Layout>
      <Outlet />
    </Layout>
  )
}
