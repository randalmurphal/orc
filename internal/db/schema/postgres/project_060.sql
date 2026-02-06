-- Project database migration 060: Remove cross-database FK on workflow_run_phases
-- The phase_template_id FK references phase_templates which lives in GlobalDB,
-- not ProjectDB. Referential integrity is enforced at the application level.

ALTER TABLE workflow_run_phases
    DROP CONSTRAINT IF EXISTS workflow_run_phases_phase_template_id_fkey;
