# Contributing to PyGo

Thanks for your interest in contributing! PyGo is designed so you can work on
one component without understanding the entire runtime.

## Quick start

```bash
git clone https://github.com/alvinjaison/PyGo
cd PyGo
go build ./...
go test ./...
```

Requires Go 1.22+.

## Development workflow

1. **Open an issue** (or comment on an existing one) before starting
   substantial work, so effort isn't duplicated.
2. **Branch** from `main`: `feat/short-desc`, `fix/issue-nnn-desc`, or
   `docs/topic`.
3. **Write tests** alongside your change. Changes without tests for new
   behavior will not be merged.
4. **Verify** before opening a PR:

   ```bash
   gofmt -l .        # should print nothing
   go vet ./...
   go test ./... -race
   ```

5. **Open a pull request** against `main` describing what changed, why, and how
   you tested it. Note any intentional divergence from CPython behavior.

## Where to start

Good first areas, roughly by prerequisite knowledge:

| Area | Prerequisites | Good for |
|------|--------------|---------|
| Standard library modules (`internal/stdlib`) | Go | Anyone familiar with the Python module |
| Object model methods (`internal/object`) | Go, Python data model | Filling in `str`/`list`/`dict` methods |
| Lexer / Parser | Go, parsing | Language-feature work |
| VM opcodes (`internal/vm`) | Go, stack VMs | Systems-minded contributors |
| Compatibility tests (`testdata/`) | Python, Go | QA-minded contributors |
| Documentation (`docs/`) | Writing | Everyone |

## Code standards

- `gofmt` is required; `go vet` must pass.
- Return `error`; do not `panic` in library code (except the `Must*` helpers).
- No `unsafe` without a documented, reviewed justification.
- Every exported type, function, and method gets a doc comment.
- `internal/` packages must not import the public `pygo` package.

The full contribution guide — including the component dependency map, the
architecture overview, and a getting-started checklist — lives in
[docs/contributing.md](docs/contributing.md). The design documents in
[docs/](docs/) explain how each component works.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By
participating, you agree to uphold it.

## License

By contributing, you agree that your contributions will be licensed under the
project's [MIT License](LICENSE).
