-- Migration: Constitution moved to file-based storage (.orc/CONSTITUTION.md)
-- No data migration needed - constitution feature was not in use

DROP TABLE IF EXISTS constitutions;

-- Keep constitution_checks for audit trail
