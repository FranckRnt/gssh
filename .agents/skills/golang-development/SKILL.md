---
name: golang-development
description: Create production-grade Go (Golang) systems with strong engineering standards, idiomatic design, and real-world architecture. Use this skill when the user asks to build APIs, CLI tools, services, concurrent systems, or backend components. Generates maintainable, well-structured, and non-generic Go code that reflects experienced engineering practices.
license: Complete terms in LICENSE.txt
---

This skill enforces production-grade Go engineering. Output must reflect senior-level judgment: explicit, testable, and free of AI boilerplate or premature abstraction.

Input: Requirement + constraints (latency, concurrency, deployment, scale).

## Engineering Thinking

Before coding, define:
- **Purpose**: Service, CLI, worker, or library? Dictates lifecycle & shutdown.
- **Scope**: Script vs. long-term system. Simplicity compounds; complexity decays.
- **Constraints**: Memory, latency, deployment target, concurrency model.
- **Trade-offs**: Explicit > clever. Measurable perf > speculative optimization.

**CRITICAL**: Every abstraction must prove its value in readability, testability, or maintainability.

---

## Architecture & Design Principles

- **Explicit Data Flow**: Pass values, avoid hidden state. If it's needed, pass it. If it mutates, name it clearly.
- **Interface Direction**: Define interfaces where callers need them, not where implementers expose them.
- **Stdlib First**: Prefer `log/slog`, `context`, `errors`, `io/fs`, `net/http` (1.22+). Justify any third-party dependency with a measurable stdlib gap.
- **Boring & Predictable**: Code should read like a technical spec: debuggable under load, maintainable by a team, and resilient to failure.

---

## Core Engineering Rules

### Naming & Structure
- **Names**: Short, contextual, idiomatic (`userID`, `fetchUser`, `ConfigLoader`). Avoid `data`, `manager`, `handler` (except `transport/`). Good names remove comment debt.
- **Layout**: `/cmd` → `/internal/domain` → `/internal/service` → `/internal/repository` → `/internal/transport` → `/pkg` (shared only). `main.go` stays minimal. Dependency flow is strictly inward. No business logic in transport/CLI layers.
- **Generics**: Use for type-safe containers/algorithms, never for DI, business logic, or premature abstraction.

### Error & Context Handling
- Check every error. Never use `_`. Wrap with `%w` (`fmt.Errorf("fetch: %w", err)`). Use `errors.Join` for concurrent/aggregated failures.
- `context.Context` is always first. Never store it in structs. Propagate cancellation/deadlines to goroutines, DB, and HTTP calls.

### Concurrency & Lifecycle
- Goroutines need explicit cleanup (`context` or `done` channels). Prefer channels for coordination/backpressure. Use `sync` primitives only when profiling proves necessity.
- No shared mutable state without strict synchronization. Panic only at startup for fatal misconfiguration.

### Observability, Testing & Quality
- **Logging**: `log/slog` only. Attach `request_id`, `duration`, `error`. Levels: `INFO` (flow), `WARN` (degraded), `ERROR` (failure).
- **Testing**: Table-driven for logic. Use `t.Cleanup()` for resource teardown. Mock via interfaces/`httptest`. Deterministic, isolated, no flaky network calls.
- **Tooling**: `gofmt`, `go vet`, `golangci-lint`, `govulncheck`, `go test ./...`. All green before output.

---

## Anti-Patterns to Reject

❌ Deep nesting (use early returns)
❌ Global state, singletons, package-level vars
❌ Magic strings/secrets in logic or configs
❌ Ignoring `context` cancellation or goroutine leaks
❌ `any`/`interface{}` where concrete types or generics suffice
❌ Premature optimization without `pprof` evidence

---

## Output Requirements

- ✅ Complete, compilable, with `go.mod` & run instructions
- ✅ Production-ready: context propagation, error wrapping, graceful shutdown, structured logging
- ✅ Tested: table-driven core logic, mocked boundaries, `t.Cleanup()` teardown
- ✅ Documented: package-level godocs. Inline comments only for non-obvious invariants or perf-critical paths.

---

## Final Principle

Good Go code is simple, explicit, and maintainable. It should feel inevitable, not clever. Every path must answer: *What happens if this fails? How is it debugged in production?*