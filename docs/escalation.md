# Gas Town Escalation Protocol

> Reference for human escalation path in Gas Town

## Overview

Gas Town agents can escalate issues to the human overseer when automated
resolution isn't possible. This provides a structured channel for:

- System-threatening errors (data corruption, security issues)
- Unresolvable conflicts (merge conflicts, ambiguous requirements)
- Design decisions requiring human judgment
- Edge cases in molecular algebra that can't proceed without guidance

## Severity Levels

| Level | Priority | Description | Examples |
|-------|----------|-------------|----------|
| **CRITICAL** | P0 (urgent) | System-threatening, immediate attention | Data corruption, security breach, system down |
| **HIGH** | P1 (high) | Important blocker, needs human soon | Unresolvable merge conflict, critical bug, ambiguous spec |
| **MEDIUM** | P2 (normal) | Standard escalation, human at convenience | Design decision needed, unclear requirements |

## Escalation Command

Any agent can escalate directly using `gt escalate`:

```bash
# Basic escalation (default: MEDIUM severity)
gt escalate "Database migration failed"

# Critical escalation - immediate attention
gt escalate -s CRITICAL "Data corruption detected in user table"

# High priority escalation
gt escalate -s HIGH "Merge conflict cannot be resolved automatically"

# With additional details
gt escalate -s MEDIUM "Need clarification on API design" -m "The spec mentions both REST and GraphQL..."
```

### What happens on escalation

1. **Bead created**: An escalation bead (tagged `escalation`) is created for audit trail
2. **Mail sent**: Mail is sent to the `overseer` (human operator) with appropriate priority
3. **Activity logged**: Event logged to the activity feed for visibility

## Escalation Flow

```
Any Agent                     Overseer (Human)
    |                              |
    | gt escalate -s HIGH "msg"    |
    |----------------------------->|
    |                              |
    | [ESCALATION] msg (P1 mail)   |
    |----------------------------->|
    |                              |
    |     Reviews & resolves       |
    |                              |
    |   bd close <esc-id>          |
    |<-----------------------------|
```

## Mayor Startup Check

On `gt prime`, the Mayor automatically checks for pending escalations:

```
## PENDING ESCALATIONS

There are 3 escalation(s) awaiting human attention:

  CRITICAL: 1
  HIGH: 1
  MEDIUM: 1

  [CRITICAL] Data corruption detected (gt-abc)
  [HIGH] Merge conflict in auth module (gt-def)
  [MEDIUM] API design clarification needed (gt-ghi)

**Action required:** Review escalations with `bd list --tag=escalation`
Close resolved ones with `bd close <id> --reason "resolution"`
```

## When to Escalate

### Agents SHOULD escalate when:

- **System errors**: Database corruption, disk full, network failures
- **Security issues**: Unauthorized access attempts, credential exposure
- **Unresolvable conflicts**: Merge conflicts that can't be auto-resolved
- **Ambiguous requirements**: Spec is unclear, multiple valid interpretations
- **Design decisions**: Architectural choices that need human judgment
- **Stuck loops**: Agent is stuck and can't make progress

### Agents should NOT escalate for:

- **Normal workflow**: Regular work that can proceed without human input
- **Recoverable errors**: Transient failures that will auto-retry
- **Information queries**: Questions that can be answered from context

## Molecular Algebra Edge Cases

All edge cases in molecular algebra should escalate rather than fail silently:

```go
// Example: Molecule step has conflicting dependencies
if hasConflictingDeps {
    // Don't fail silently - escalate
    exec.Command("gt", "escalate", "-s", "HIGH",
        "Molecule step has conflicting dependencies: "+stepID).Run()
}
```

This ensures:
1. Issues are visible to humans
2. Audit trail exists for debugging
3. System doesn't silently break

## Viewing Escalations

```bash
# List all open escalations
bd list --status=open --tag=escalation

# View specific escalation
bd show <escalation-id>

# Close resolved escalation
bd close <id> --reason "Resolved by fixing X"
```

## Integration with Existing Flow

The escalation system integrates with the existing polecat exit flow:

| Exit Type | When to Use | Escalation? |
|-----------|-------------|-------------|
| `COMPLETED` | Work done successfully | No |
| `ESCALATED` | Hit blocker, needs human | Yes (automatic via `gt done --exit ESCALATED`) |
| `DEFERRED` | Work paused, will resume | No |

When a polecat uses `gt done --exit ESCALATED`:
1. Witness receives the notification
2. Witness can forward to Mayor with `ESCALATION:` subject
3. Mayor callback handler forwards to overseer

The new `gt escalate` command provides a more direct path that any agent can use,
with structured severity levels and audit trail.
