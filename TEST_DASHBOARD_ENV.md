# Dashboard Environment Variable Test

The issue is that `NEXT_PUBLIC_*` environment variables need to be provided at **build time** for Next.js to embed them into the client-side JavaScript bundle.

## Quick Fix

Run this command to test if the API is accessible from the browser:

```bash
# Open browser console at http://localhost:3000/launch and run:
fetch('http://localhost:8080/admin/models/r2', {
  headers: {
    'X-Admin-Token': 'df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a'
  }
})
.then(r => r.json())
.then(d => console.log('Models:', d.models.length))
.catch(e => console.error('Error:', e));
```

## Permanent Solution

The build args in docker-compose.yml reference environment variables that should be loaded from `.env`:

```yaml
build:
  context: .
  dockerfile: Dockerfile.dashboard
  args:
    - NEXT_PUBLIC_API_URL=http://localhost:8080
    - NEXT_PUBLIC_ADMIN_TOKEN=${ADMIN_API_TOKEN}
```

The `${ADMIN_API_TOKEN}` will be substituted from the `.env` file when running `docker compose build`.

## Manual Build Command

```bash
cd /Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas

# Build with explicit build args
docker compose build \
  --build-arg NEXT_PUBLIC_API_URL=http://localhost:8080 \
  --build-arg NEXT_PUBLIC_ADMIN_TOKEN=df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a \
  dashboard

# Restart
docker compose up -d dashboard
```

## Verification

After rebuild, check if environment variables are embedded:

```bash
# Check built JavaScript for the API URL
docker run --rm crosslogic-ai-iaas-dashboard cat /.next/static/chunks/app/launch*.js | grep -o "http://localhost:8080" | head -1
```

