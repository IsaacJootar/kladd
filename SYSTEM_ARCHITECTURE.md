# KLADD — System Architecture

## Main Modules

1. User Account Module
- authentication
- registration
- sessions
- profile management
- account security
- Security PIN setup and reset

2. Evidence Vault
- uploaded evidence
- metadata
- verification state
- evidence relationships

3. Identity Core
- BVN
- NIN
- Tax IDs
- Passport numbers
- tokenized internal references

4. Evidence Verification Engine
- OCR
- validation
- cross-checking
- institution verification
- expiry checks

5. Truth Registry & Truth Engine
- truth keys
- derivation rules
- sensitivity
- return types
- validity rules

6. Claim Request Module
- request truths
- define purpose
- define scope
- define duration

7. Consent & Approval Module
- request review
- Security PIN validation
- approval records
- denial records
- consent audit logging

No claim may be issued without successful approval.

8. Claim Issuing Engine
- signed claims
- verification tokens
- expiry rules
- retrieval references

9. Claim Exchange Network
- API exchange
- verification pages
- QR exchange
- PIN exchange
- webhook delivery

10. Audit & Logging Module
- requests
- approvals
- retrievals
- expiries
- revocations
- failed PIN attempts
