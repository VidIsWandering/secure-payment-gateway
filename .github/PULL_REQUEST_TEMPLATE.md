## Type of change

Please delete options that are not relevant.

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] This change requires a documentation update

## Description

Provide a brief overview of what this PR does. Be clear and specific. 

## Related Issues

Resolves/Fixes #IssueNumber

## Database Schema Changes

- [ ] Does this PR alter the database schema?
  - If yes, verify `migrate-up` succeeded and migrations are stored properly in `db/migrations/`. 
  - Mention any `FOR UPDATE` table locks introduced in this change for Transaction safety.

## Checklist

- [ ] My code follows the Clean Architecture structure denoted in `PROJECT_STRUCTURE.md`.
- [ ] Core business layer (`internal/core`) does not import external HTTP/Storage dependencies.
- [ ] I have executed tests using `make test`. All local tests passed.
- [ ] My code respects the high concurrency locking logic required for Wallet adjustments.
- [ ] I have provided test coverage for any new or modified Logic in `internal/service`.

## Testing Environment

- OS: [e.g. macOS/Linux/Windows]
- Docker version: [e.g. 24.x]
- PostgreSQL Version: [e.g. 16.0]
