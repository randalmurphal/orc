# ADR-005: Task Weight System

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Different tasks require different levels of rigor. A one-size-fits-all approach either over-engineers simple tasks or under-engineers complex ones.

## Decision

**Five weight levels** that determine phase templates:

| Weight | Description | Phases | Example |
|--------|-------------|--------|---------|
| `trivial` | <10 line change | implement | Typo fix, config tweak |
| `small` | Single component | implement → test | Bug fix, add field |
| `medium` | Multiple files | spec → implement → review → test | Feature, refactor |
| `large` | Cross-cutting | research → spec → design → implement → review → test → validate | Major feature |
| `greenfield` | New system | research → spec → design → scaffold → implement → test → validate → docs | New service |

**AI classification is the first phase** for all tasks. User can override.

## Rationale

### Why Five Levels?

- `trivial` is important because many tasks need zero ceremony
- `greenfield` is important because new systems need extra research
- Fewer levels don't capture the extremes; more creates decision paralysis

### Classification Signals

| Signal | Weight Suggestion |
|--------|-------------------|
| 1-2 files mentioned | trivial/small |
| 3-10 files | small/medium |
| 10+ files | large/greenfield |
| "fix", "typo", "bump" | trivial |
| "refactor", "redesign" | medium/large |
| "new service", "from scratch" | greenfield |
| Database changes mentioned | +1 level |
| Breaking changes mentioned | +1 level |

**When in doubt, round UP.** Better to over-prepare.

## Consequences

**Positive**:
- Right-sized process: trivial stays trivial, complex gets rigor
- Predictable: users know what to expect per weight
- Fast start: AI classifies instantly

**Negative**:
- Misclassification risk
- Five templates to maintain

**Mitigation**: Easy override with `--weight`; templates inherit from base.
