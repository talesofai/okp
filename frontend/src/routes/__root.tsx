import { Outlet, createRootRoute } from '@tanstack/react-router'
import { Text } from '@cloudflare/kumo'

export const Route = createRootRoute({
  component: Root,
})

function Root() {
  return (
    <div style={{ minHeight: '100vh' }}>
      <nav className="bg-kumo-surface-elevated border-b border-kumo-border" style={{
        display: 'flex', alignItems: 'baseline', gap: 12,
        padding: '16px 24px',
      }}>
        <a href="/" style={{ textDecoration: 'none' }}>
          <Text size="lg" weight="bold">okp</Text>
        </a>
        <Text color="secondary" size="sm">open knowledge pool</Text>
      </nav>
      <main style={{ maxWidth: 960, margin: '0 auto', padding: '32px 24px' }}>
        <Outlet />
      </main>
    </div>
  )
}
