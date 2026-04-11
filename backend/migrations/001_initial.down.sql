-- 001_initial.down.sql
-- Reverses 001_initial.up.sql
-- Drop order respects foreign-key dependencies (dependents before referenced tables).

DROP TABLE IF EXISTS referrals;
DROP TABLE IF EXISTS fraud_events;
DROP TABLE IF EXISTS customer_usage;
DROP TABLE IF EXISTS claimable_rewards;
DROP TABLE IF EXISTS earnings;
DROP TABLE IF EXISTS metering_default;   -- partition must be dropped before parent
DROP TABLE IF EXISTS metering;
DROP TABLE IF EXISTS nodes;
DROP TABLE IF EXISTS customers;
