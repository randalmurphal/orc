-- Migration 046: Add extract field to workflow_variables
-- Adds support for JSONPath extraction from resolved variable values.
-- The extract field contains a gjson path expression applied after resolution.

ALTER TABLE workflow_variables ADD COLUMN extract TEXT;
