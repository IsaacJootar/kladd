# KLADD — Consent & Approval Specification

## Purpose
Approval is not only a button click. It is a security event where the user reviews a request, confirms release using a Security PIN, and creates an auditable consent record.

## MVP Approval Method
Kladd MVP uses one approval method: Security PIN.

## Security PIN Rules
- 4–6 digits
- stored hashed, never raw
- required for every claim approval
- failed attempts are rate limited
- repeated failed attempts temporarily lock approval
- PIN reset requires account re-authentication

Recommended lock rule:
- 5 failed PIN attempts
- lock approval attempts for 15 minutes

## Approval Flow
Request arrives → user opens request → user reviews requester, purpose, truths, and duration → user clicks Approve → Kladd asks for Security PIN → user enters PIN → Kladd validates PIN → consent record created → claim issued.

## Consent Record
```json
{
  "consent_id": "cns_001",
  "claim_request_id": "req_001",
  "claim_id": "clm_001",
  "user_id": "usr_001",
  "organization_id": "org_001",
  "approved": true,
  "approval_method": "security_pin",
  "approved_at": "2026-06-01T12:00:00Z",
  "ip_address": "102.xxx.xxx.xxx",
  "user_agent": "browser/device info",
  "session_id": "ses_001"
}
```

## Enforcement Rule
No claim may be issued unless:
- the request exists
- the request is valid
- the user approved it
- the Security PIN was successfully validated
- the consent record was created
