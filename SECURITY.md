# Security Policy

## Supported Versions

PyGo is pre-1.0 and under active development. Security fixes are applied to the
`main` branch. There are no long-term support branches yet.

## Reporting a Vulnerability

Please report security vulnerabilities **privately** rather than opening a
public issue.

- Use GitHub's [private vulnerability reporting][gh-report] for this repository
  ("Security" tab → "Report a vulnerability"), or
- Contact a maintainer directly.

Please include a description, reproduction steps, and the impact you observed.
We aim to acknowledge reports within a few days.

[gh-report]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability

## Scope and Security Model

**PyGo is not a hardened sandbox by default.** Read
[docs/security.md](docs/security.md) for the full threat model before running
untrusted code. Key points:

- A Python script can only reach Go capabilities the host explicitly registers
  via `RegisterFunction`, `Register`, or `RegisterModule`.
- For untrusted input, use `WithRestrictedBuiltins()` (disables `open`, `os`,
  `exec`/`eval`) together with execution limits: `WithMaxInstructions`,
  `WithMaxMemory`, and `WithExecutionTimeout` / context cancellation. These
  limits are uncatchable aborts — a script cannot swallow them with `except`.

We are most interested in reports of:

- Inputs that panic or crash the interpreter (parser, compiler, or VM).
- Ways to escape `WithRestrictedBuiltins()` or defeat the execution limits.
- Memory-safety issues in the interop layer.
