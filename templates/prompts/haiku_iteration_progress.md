Evaluate whether an AI agent's work is progressing toward the success criteria.

## Specification
{{.SpecContent}}

## Agent's Latest Output
{{.IterationOutput}}

## Task
Assess if the work is:
- ON TRACK: Making progress toward success criteria → decision: "CONTINUE"
- OFF TRACK: Wrong approach, scope creep, misunderstanding → decision: "RETRY"
- BLOCKED: Missing dependencies, impossible requirements → decision: "STOP"
