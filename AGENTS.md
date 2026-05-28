# AGENTS.md - Kladd Project Instructions

You are helping build Kladd.

Kladd is a reusable proof, verification, and claim exchange platform.

Core principle:

Verify once. Prove everywhere.

Before coding:
- read all markdown specification files in the project root
- follow architecture documents strictly
- do not invent a new architecture
- do not over-engineer
- keep the system modular and simple

## Preferred Stack

Frontend:
- TypeScript
- Next.js
- Tailwind CSS
- shadcn/ui

Backend:
- Go

Database:
- PostgreSQL

Cache / Queue:
- Redis

Storage:
- S3-compatible object storage

## Important Product Rules

Kladd is NOT:
- a file sharing platform
- unrestricted document transfer
- a cloud drive

Kladd IS:
- a reusable proof system
- a truth exchange network
- a claim verification platform

## Core Security Rules

- no raw sensitive document exposure by default
- every claim approval requires Security PIN validation
- expired claims must hide truth details
- revoked claims must hide truth details
- all sensitive actions must be audited

## Security PIN Rules

- 4-6 digits
- hashed only
- never logged
- rate limited
- lock after repeated failures

## Development Workflow

1. Build one module at a time.
2. After completing a module:
   - run checks/tests
   - summarize completed work
   - create a git commit
   - stop and ask for approval
3. Do not continue to the next module without approval.

## Commit Style

Use clean module-based commits.

Examples:
- feat(auth): implement user authentication
- feat(pin): implement Security PIN approval flow
- feat(claims): implement claim issuing engine
- feat(vault): implement evidence vault module

## UI/UX Direction

Kladd UI must feel:
- calm
- modern
- minimal
- trustworthy
- mobile-friendly

Avoid:
- technical overload
- crypto aesthetics
- enterprise clutter
- identity bureaucracy feeling

Users should feel:
"I control my proofs."

## Engineering Principles

- simplicity first
- explicit workflows
- predictable behavior
- avoid speculative abstractions
- avoid unnecessary configuration
- preserve architectural consistency

If architecture ambiguity exists:
- ask before implementing
