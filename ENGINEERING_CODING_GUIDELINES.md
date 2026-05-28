# KLADD — Engineering & Coding Guidelines

## Purpose
This document defines engineering behavior, implementation discipline, and coding philosophy expected while building Kladd.

## 1. Think Before Coding
Never implement blindly.
- state assumptions clearly
- identify uncertainty
- ask for clarification when architecture is ambiguous
- avoid silent interpretation changes

## 2. Simplicity First
Do not over-engineer.
Avoid speculative flexibility, unnecessary configuration, and generic abstractions for single-use logic.

## 3. Surgical Changes
Modify only what is necessary.
Do not refactor unrelated systems.

## 4. Goal-Driven Engineering
Every task should define:
- expected outcome
- verification method
- success condition

## 5. Kladd Architectural Principles
Kladd is not a file sharing platform.
Kladd is a reusable proof system, truth exchange network, and claim verification platform.

## 6. Evidence Exposure Rules
Never expose raw evidence unless explicitly required.
Prefer truth outputs, verification status, operational claims, and masked references.

## 7. Claim Rules
Every claim must:
- belong to a valid request
- require explicit consent approval
- require Security PIN validation
- have expiration
- support revocation
- be auditable

Expired or revoked claims must not expose truth details.

## 8. Security PIN Rules
Security PIN:
- is mandatory for claim approval
- must be hashed
- must never be logged
- must be rate-limited
- must support temporary lockouts

Security PIN is not an exchange PIN or public retrieval code.

## 9. Truth Engine Discipline
No truth should exist without:
- derivation rules
- sensitivity classification
- validity duration
- allowed purposes
- required evidence

## 10. UI/UX Discipline
Kladd UI should feel calm, modern, minimal, trustworthy, and frictionless.

## 11. Logging & Auditability
Log approvals, denials, claim retrievals, revocations, PIN failures, admin actions, and verification access.

Do not log raw Security PINs, full sensitive identifiers, or unrestricted evidence payloads.

## 12. AI-Assisted Development Rule
AI tools should implement architecture, not invent architecture.
Do not allow uncontrolled architectural drift.
