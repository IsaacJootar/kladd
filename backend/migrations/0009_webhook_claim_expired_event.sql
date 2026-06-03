ALTER TABLE webhook_deliveries
DROP CONSTRAINT webhook_deliveries_event_type_known;

ALTER TABLE webhook_deliveries
ADD CONSTRAINT webhook_deliveries_event_type_known
CHECK (event_type IN ('claim.approved', 'claim.expired', 'claim.revoked'));
