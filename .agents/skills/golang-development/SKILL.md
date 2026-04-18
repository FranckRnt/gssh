---
name: golang-development
description: Create production-grade Go (Golang) code with strong engineering standards, idiomatic design, and real-world architecture. Use this skill when the user asks to build APIs, CLI tools, services, concurrent systems, or backend components. Generates maintainable, well-structured, and non-generic Go code that reflects experienced engineering practices.
license: Complete terms in LICENSE.txt
--------------------------------------

This skill guides the creation of production-grade Go systems that avoid generic, toy-like, or AI-generated code patterns. The goal is to produce code that looks like it was written by an experienced Go engineer in a real company.

The user provides a requirement: API, CLI, service, agent, or backend component. They may include constraints (performance, concurrency, infra, etc.).

## Engineering Thinking

Before coding, understand the problem deeply:

* Purpose: CLI, service, worker, reusable package?
* Scope: script vs production system
* Constraints: concurrency, memory, latency, deployment
* Trade-offs: simplicity vs flexibility, performance vs clarity

CRITICAL: Avoid overengineering AND naive implementations.

---

## Code Design Philosophy

Write code that feels:

* Idiomatic Go
* Simple and explicit
* Readable and maintainable
* Structured and intentional

Avoid:

* Clever abstractions
* Overuse of interfaces
* Generic names (data, manager, handler)
* Deep nesting

Prefer:

* Small functions
* Clear responsibilities
* Explicit data flow

---

## Naming & Semantics

Names must be short, meaningful, and idiomatic:

* Variables: userID, retryCount, configPath
* Functions: fetchUser, computeHash, validateInput
* Structs: UserService, PaymentProcessor, ConfigLoader
* Interfaces: Reader, UserRepository

Avoid:

* data, obj, thing, manager
* meaningless abbreviations

CRITICAL: Good naming removes the need for comments.

---

## Project Structure

/cmd/app            → entrypoint
/internal/domain    → business models
/internal/service   → business logic
/internal/repository→ data access
/internal/transport → HTTP / gRPC / CLI
/pkg                → reusable packages
/configs            → config files

Rules:

* main.go must stay minimal
* Business logic must not live in handlers
* Avoid monolithic packages

---

## Error Handling

* Always check errors
* Never ignore errors (_)
* Wrap errors with context

Example:

return fmt.Errorf("fetch user: %w", err)

* No panic except at startup

---

## Context Usage

* Always pass context.Context as first parameter
* Never store context in struct
* Respect cancellation

---

## Concurrency

* Use goroutines intentionally
* Avoid leaks
* Use channels for coordination
* Use sync primitives only when necessary

---

## Logging & Observability

* Use structured logging (log/slog)
* Never use fmt.Println in production
* Include contextual fields (request_id, etc.)

---

## Testing Strategy

* Use testing package
* Table-driven tests
* Mock via interfaces
* No flaky tests

---

## API & CLI

* Validate inputs early
* Separate handler / service
* No business logic in CLI layer

---

## Performance

* Avoid premature optimization
* Benchmark when needed
* Use pprof

---

## Security

* Validate inputs
* No hardcoded secrets
* Use env/config

---

## Tooling

* gofmt
* go vet
* golangci-lint

---

## Output Requirements

* Complete code
* Imports included
* Compilable
* Tests when relevant

---

## Final Principle

Good Go code is simple, explicit, and maintainable.

CRITICAL: Would a senior Go engineer approve this?
