-- Quality metrics tracking for phase retries, review rejections, and manual interventions

-- Quality metrics (JSON object with phase_retries, review_rejections, manual_intervention, etc.)
ALTER TABLE tasks ADD COLUMN quality TEXT;
