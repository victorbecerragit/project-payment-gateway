-- Migration: 002_alter_payments_amount_type
-- Changes amount from cents (BIGINT) to dollars/decimal (NUMERIC)
-- to support floating point values directly from the application.

ALTER TABLE payments ALTER COLUMN amount TYPE NUMERIC(10,2);