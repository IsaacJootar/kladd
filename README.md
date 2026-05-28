# Kladd

## Verify once. Prove everywhere.

Kladd is a reusable proof, verification, and claim exchange platform that replaces repetitive document uploads with controlled, time-bound truth verification.

Instead of repeatedly uploading sensitive documents to banks, employers, schools, government portals, healthcare providers, and platforms, users verify records once and approve reusable claims whenever proof is needed.

Organizations receive only the approved truths they need - not unrestricted access to raw documents.

---

# Core Idea

Traditional verification systems operate through:

* document duplication,
* unrestricted file transfer,
* repeated uploads,
* and permanent exposure of sensitive records.

Kladd introduces a different model:

```text id="3x6mwe"
Request -> Approval -> Claim -> Verification -> Expiry
```

Users:

* upload and verify records once,
* approve access intentionally,
* control who sees what,
* revoke access,
* and track verification history.

Organizations:

* request operational truths,
* receive signed time-bound claims,
* verify status,
* and avoid handling unnecessary sensitive evidence.

---

# Core Concepts

## Evidence

Protected records uploaded or linked by users.

Examples:

* passport
* degree certificate
* CAC document
* utility bill
* license
* tax document

---

## Truths

Reusable operational verification outputs derived from evidence.

Examples:

```text id="4xch1f"
identity_verified
age_over_18
degree_verified
business_registered
address_verified
bvn_verified
license_active
```

Truths answer verification questions without exposing unnecessary raw data.

---

## Claims

Time-bound verification packages issued after user approval.

Claims contain:

* approved truths,
* requester,
* purpose,
* expiry,
* verification status,
* and retrieval references.

Claims are:

* revocable,
* auditable,
* and temporary.

Expired or revoked claims no longer expose truth details.

---

# Main Product Principles

* Verify once
* Controlled truth exchange
* Minimal exposure
* User-controlled access
* Time-bound verification
* Revocable claims
* Auditability
* Reusable trust

---

# Example Workflow

## Employment Verification

```text id="x8apcz"
Company requests:
- identity_verified
- degree_verified

v

User reviews request

v

User approves with Security PIN

v

Kladd issues active claim

v

Company verifies approved truths

v

Claim expires automatically
```

The company never receives:

* passport copy
* transcript file
* unrestricted document access

---

# Exchange Mechanisms

Kladd supports multiple exchange methods.

## Verify with Kladd

Embedded verification button for forms and onboarding systems.

---

## Claim Links

Temporary verification URLs.

---

## QR Verification

Fast verification for physical environments.

Examples:

* hospitals
* schools
* government counters
* visitor access

---

## PIN Exchange

Temporary exchange PINs for lightweight verification.

---

## API & Webhook Exchange

Machine-to-machine verification for integrated systems.

---

# Security Model

Kladd is designed around controlled access.

## Security PIN Approval

Every claim release requires explicit approval using a Security PIN.

Approval is treated as:

* a consent event,
* a security event,
* and an auditable action.

---

## Expiry Rules

Expired claims:

* remain auditable,
* but no longer expose truth details.

---

## Sensitive Identity Handling

Critical identifiers:

* BVN
* NIN
* passport numbers
* tax IDs

are:

* isolated,
* encrypted,
* and never exposed by default.

---

# MVP Modules

* User accounts
* Evidence vault
* Truth registry
* Organization dashboard
* Claim requests
* Consent & Security PIN approval
* Claim issuing
* Verification pages
* Audit logs
* QR/PIN exchange
* API & webhooks

---

# Product Vision

Kladd aims to become a reusable verification infrastructure layer where:

```text id="uh8r7e"
Organizations request truths,
not documents.
```

and users maintain visibility and control over their proofs.

---

# Guiding Principle

```text id="q9w3lp"
Verify once. Prove everywhere.
```
