# KLADD — UI/UX Flows

## User Dashboard
- My Evidence
- Active Claims
- Pending Requests
- Access History
- Security

## Organization Dashboard
- Create Request
- Pending Approvals
- Active Claims
- Expired Claims
- Audit Logs

## Security PIN Setup
User creates account → user goes to Security → user sets 4–6 digit Security PIN → PIN is confirmed → PIN hash is stored.

## Approval Flow
User sees requester, purpose, requested truths, duration, and raw document protection note.
User clicks Approve → Security PIN modal opens → user enters PIN → Kladd validates PIN → claim is issued.

## Verification Page
Active state shows truths, status, and expiry.
Expired/revoked state shows metadata only, no truth details.

## QR Flow
Generate QR → staff scans → verification page opens.

## PIN Exchange Flow
Generate temporary exchange PIN → staff enters PIN → verification page opens.
