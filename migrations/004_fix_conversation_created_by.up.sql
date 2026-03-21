-- Drop the NOT NULL constraint on created_by so ON DELETE SET NULL can work.
-- When a user account is deleted, created_by becomes NULL rather than causing
-- a constraint violation.
ALTER TABLE conversations ALTER COLUMN created_by DROP NOT NULL;
