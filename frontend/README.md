# CrossLogic Frontend

Modern, professional frontend for CrossLogic GPU IaaS Platform built with Vite + React + TypeScript + shadcn/ui.

## Features

- **Modern Stack**: Vite, React 19, TypeScript, TanStack Router, TanStack Query
- **Professional UI**: shadcn/ui components with Tailwind CSS
- **Developer-Oriented**: Clean, technical design appealing to engineers
- **Fully Responsive**: Mobile, tablet, and desktop optimized
- **Real-time Updates**: React Query for automatic data refresh
- **Type-Safe**: Full TypeScript coverage with proper API types

## Pages

1. **Dashboard** - Usage metrics, node status, quick start guide
2. **Launch** - Deploy AI models on cloud GPUs
3. **API Keys** - Manage authentication keys
4. **Usage & Billing** - Track token usage and costs with charts
5. **Nodes** - Manage active GPU instances
6. **Settings** - Configure preferences

## Development

### Prerequisites

- Node.js 20+
- npm or pnpm

### Local Development

```bash
# Install dependencies
npm install

# Copy environment variables
cp .env.example .env
# Edit .env with your API URL and admin token

# Start development server
npm run dev
```

The app will be available at http://localhost:5173

### Build

```bash
# Production build
npm run build

# Preview production build
npm run preview
```

## Docker Deployment

The frontend is designed to run in Docker:

```bash
# Build and run with docker-compose
docker-compose up --build frontend

# Or build Docker image directly
docker build -f Dockerfile.frontend -t crosslogic-frontend .
docker run -p 3000:80 crosslogic-frontend
```

Access the frontend at http://localhost:3000

## Environment Variables

- `VITE_API_BASE_URL` - Control plane API URL (default: http://localhost:8080)
- `VITE_ADMIN_TOKEN` - Admin authentication token

## Architecture

```
frontend/
├── src/
│   ├── components/
│   │   ├── ui/          # shadcn/ui primitives
│   │   ├── layout/      # Sidebar, Topbar, Layout
│   │   └── common/      # Reusable components (StatCard, StatusBadge)
│   ├── routes/          # TanStack Router pages
│   │   ├── __root.tsx
│   │   ├── _authenticated.tsx
│   │   ├── login.tsx
│   │   └── _authenticated/
│   │       ├── index.tsx      # Dashboard
│   │       ├── launch.tsx     # Launch instances
│   │       ├── api-keys.tsx   # API key management
│   │       ├── usage.tsx      # Usage & billing
│   │       ├── nodes.tsx      # Node management
│   │       └── settings.tsx   # Settings
│   ├── lib/
│   │   ├── api.ts       # API client and endpoints
│   │   └── utils.ts     # Utility functions
│   ├── stores/
│   │   └── auth.ts      # Zustand auth store
│   ├── types/
│   │   └── index.ts     # TypeScript types
│   └── styles/
│       └── globals.css  # Global styles
├── Dockerfile.frontend  # Production Docker build
└── nginx.conf          # Nginx configuration for serving
```

## Design System

Colors, typography, spacing, and component styles follow the design specification in `/DESIGN_SPECIFICATION.md`.

### Key Design Principles

- **Professional**: Looks like an enterprise SaaS product
- **Developer-Friendly**: Technical without being cold
- **Accessible**: WCAG 2.1 AA compliant
- **Responsive**: Mobile-first design
- **Performant**: Optimized bundles, lazy loading

## API Integration

The frontend integrates with the CrossLogic Control Plane API:

- All API calls use Axios with interceptors
- Authentication via X-Admin-Token header
- React Query for caching and real-time updates
- Type-safe API responses with TypeScript

## Authentication

Simple token-based authentication:

1. User enters admin token on login page
2. Token stored in localStorage
3. Added to all API requests via Axios interceptor
4. Protected routes check authentication status
5. Auto-redirect to login if unauthenticated

## Contributing

1. Follow the existing code style
2. Use TypeScript strictly - no `any` types
3. Create reusable components in `components/common`
4. Update types when adding new API endpoints
5. Test on mobile, tablet, and desktop
6. Run `npm run lint` before committing

## License

Copyright CrossLogic. All rights reserved.
