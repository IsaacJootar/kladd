# KLADD — Truth Registry Specification

## Purpose
The Truth Registry defines:
- supported truths
- derivation rules
- allowed purposes
- exposure rules
- validity durations

## Categories
- Identity
- Age
- Address
- Education
- Employment
- Business
- Tax
- Banking
- Government
- Licensing
- Healthcare
- Compliance

## Example Truth Object
```json
{
  "truth_key": "age_over_18",
  "category": "age",
  "return_type": "boolean",
  "sensitivity": "low",
  "validity_days": 365,
  "required_evidence": ["verified_date_of_birth"]
}
```

## Return Types
- boolean
- enum
- range
- status
- masked_value
- date

## Rules
- no truth without derivation rule
- no truth without validity duration
- no unrestricted truth exposure
- every truth must have sensitivity level
- truths are only released inside approved claim sessions
