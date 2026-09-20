# Workspace

## Overview

pnpm workspace monorepo using TypeScript. Each package manages its own dependencies.

## Stack

- **Monorepo tool**: pnpm workspaces
- **Node.js version**: 24
- **Package manager**: pnpm
- **TypeScript version**: 5.9
- **API framework**: Express 5
- **Database**: PostgreSQL + Drizzle ORM
- **Validation**: Zod (`zod/v4`), `drizzle-zod`
- **API codegen**: Orval (from OpenAPI spec)
- **Build**: esbuild (CJS bundle)

## Key Commands

- `pnpm run typecheck` — full typecheck across all packages
- `pnpm run build` — typecheck + build all packages
- `pnpm app-dev` — start the WhisperLedger mobile app
- `pnpm api-dev` — run API server locally
- `pnpm --filter @whisperledger/api-spec run codegen` — regenerate API hooks and Zod schemas from OpenAPI spec
- `pnpm --filter @whisperledger/db run push` — push DB schema changes (dev only)

## Artifacts

### WhisperLedger (Mobile App)
- **Path**: `whisperledger-app/`
- **Type**: Expo (React Native)
- **Features**: AI SMS Parsing, Biometric Lock, Cloud Sync, Theme support.
- **Storage**: Firebase + AsyncStorage

### Admin Dashboard
- **Canonical path**: `../whisperledger-website/admin/`
- **Type**: HTML/CSS/JavaScript with Firebase Authentication and Firestore listeners
- **Purpose**: Notifications, user controls, behavior insights, live user monitoring, and support case management

### API Server
- **Path**: `api-server/`
- **Type**: Node.js/Express
