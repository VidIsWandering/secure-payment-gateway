# Contributing to Secure Payment Gateway

First off, thank you for considering contributing to the Secure Payment Gateway! It's people like you that make the open source community such a great place to learn, inspire, and create.

## How to Contribute

### 1. Prerequisites

Before you start, make sure you have installed:
- Go 1.25+
- Docker and Docker Compose
- Make

### 2. Setting Up Your Development Environment

1. Fork the repository on GitHub.
2. Clone your forked repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/secure-payment-gateway.git
   ```
3. Navigate to the project directory:
   ```bash
   cd secure-payment-gateway
   ```
4. Start the required services (PostgreSQL, Redis):
   ```bash
   docker compose up -d postgres redis
   ```
5. Apply database migrations:
   ```bash
   make migrate-up
   ```

### 3. Making Changes

- Create a new branch for your feature or bug fix:
  ```bash
  git checkout -b feature/your-feature-name
  ```
- Make your changes. Ensure you adhere to the project's **Clean Architecture** guidelines as outlined in `PROJECT_STRUCTURE.md`.
- Write unit tests for any new logic. If changing core transaction handling, verify with integration tests.

### 4. Code Quality and Testing

Before submitting a Pull Request, you must run the following checks. All checks must pass.

- Run the linter to ensure code style consistency:
  ```bash
  make lint
  ```
- Run the test suite:
  ```bash
  make test
  ```
- To run with coverage:
  ```bash
  make coverage
  ```

### 5. Commit Standards

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification. This helps us automate generating changelogs.
- `feat:` A new feature
- `fix:` A bug fix
- `docs:` Documentation only changes
- `style:` Changes that do not affect the meaning of the code
- `refactor:` A code change that neither fixes a bug nor adds a feature
- `perf:` A code change that improves performance
- `test:` Adding missing tests or correcting existing tests
- `chore:` Changes to the build process or auxiliary tools

### 6. Submitting a Pull Request

- Push your changes to your fork.
- Open a Pull Request against the `main` branch of the upstream repository.
- Ensure your Pull Request description explains **what** the changes do, **why** they are necessary, and fills out the provided PR template.

## Architectural Guidelines Reminder

When contributing to core services (especially `payment_service.go`), please remember the project's stringent requirements:
1. **Never** import external framework libraries into `internal/core`.
2. All wallet balance updates **must** be executed within a `pgx.Tx` transaction utilizing Pessimistic Locking (`FOR UPDATE`).
3. Ensure idempotency operations interact correctly with the Redis cache fallback chain.

Once your PR is submitted, project maintainers will review the CI test results and leave feedback. Thank you for your contribution!
