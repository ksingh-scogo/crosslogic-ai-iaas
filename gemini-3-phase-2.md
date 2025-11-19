# Phase 2: Dashboard & User Experience - Implementation Progress

**Status**: In Progress
**Start Date**: January 2025

## Goals
Provide a functional UI for users to manage their API keys and view usage, and for admins to manage nodes.

## Implementation Steps

### 1. Backend Prerequisites (Control Plane)
- [x] **Verify/Add API Key Management Endpoints**: Added `api_keys.go` with handlers.
- [x] **Register Endpoints**: Need to register routes in `gateway.go`.
- [x] **Verify/Add Tenant Management Endpoints**: Ensure endpoints exist to create/get tenants (needed for auth flow).

### 2. Dashboard Skeleton & Authentication
- [x] **Setup NextAuth.js**: Configure Google Provider and Credentials provider (for dev).
- [x] **Session Management**: Ensure session persists and contains Tenant ID.

### 3. API Client Integration
- [x] **Update `lib/api.ts`**: Implement functions to call Control Plane Admin API using the server-side Admin Token.

### 4. Feature: API Key Management
- [x] **List Keys Page**: UI to list existing keys.
- [x] **Create Key Modal**: UI to generate new keys.
- [x] **Revoke Key Action**: UI to delete/revoke keys.

### 5. Feature: Usage Visualization
- [x] **Install Recharts**: Add charting library.
- [x] **Usage Page**: Visualize `usage_hourly` data.

### 6. Feature: Node Management (Admin)
- [x] **Node List**: View active nodes and status.
- [x] **Launch/Terminate**: UI controls for SkyPilot.

## Progress Log
- **[Date]**: Initialized plan.
- **[Date]**: Created `api_keys.go` with handler functions.
- **[Date]**: Implemented Dashboard UI features (API Keys, Usage, Nodes).
