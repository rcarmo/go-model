This project is GoModel — a high-performance, lightweight AI gateway that routes requests to multiple AI model providers through an OpenAI-compatible API.

## Core Principles

**Follow Postel’s Law.**

- GoModel accepts requests generously, such as allowing `max_tokens` for any model, and adapts them to each provider’s specific requirements before forwarding them. For example, it translates `max_tokens` to `max_completion_tokens` for OpenAI reasoning models.
- GoModel accepts provider responses liberally and returns them to the user in a conservative OpenAI-compatible format.

**Follow [The Twelve-Factor App](https://12factor.net/).**

Keep files small and follow KISS principles.

Keep the implementation explicit and maintainable rather than relying on clever abstractions.

## Project cache/temp policy

Canonical project name: `go-model`.

Resolve the disposable root once before exporting child temp/cache variables:

1. If `PROJECT_TMP_BASE` is set, append `/go-model`; the base must be absolute and the resulting root usable. If `PROJECT_TMP_ROOT` is also set, both must resolve to the same root.
2. Else if `PROJECT_TMP_ROOT` is set, it must be an absolute usable directory path ending in `/go-model`; invalid explicit overrides fail.
3. Else CI uses `${RUNNER_TEMP}/go-model`, then the original `${TMPDIR}/go-model`, then the platform temp directory plus `/go-model`, even if `/workspace/tmp` exists.
4. Else local use prefers writable `/workspace/tmp/go-model`, then platform temp plus `/go-model`.

Use `scripts/project-tmp.sh` for repo-local resolution; it is self-contained for portable CI and mirrors `/workspace/tools/project-tmp.sh` semantics.

- Rebuildable caches: `${PROJECT_TMP_ROOT}/cache/<tool>/`.
- Generated build output/scratch: `${PROJECT_TMP_ROOT}/build/`.
- Test scratch: `${PROJECT_TMP_ROOT}/tests/`.
- Disposable logs: `${PROJECT_TMP_ROOT}/logs/`.
- Isolated run scratch: `${PROJECT_TMP_ROOT}/runs/<purpose>/<run-id>/`.
- Do not use bare `/tmp`, home-directory caches, or ad-hoc top-level workspace paths for reproducible build caches or disposable temporary files.
- Route `TMPDIR`, `GOCACHE`, `GOMODCACHE`, `GOTOOLCHAIN`, script output directories, local model cache directories, and test stack scratch through the resolved root before running tests/builds/scripts.
- `make clean` may remove project build artifacts only; do not delete retained evidence, external toolchains, or another project's temp root.

**Use good defaults.**

Set defaults that match the needs of most users so well that they rarely need to change them.

### Commit Format — Use Conventional Commits

Use the Conventional Commits format for commit subjects and PR titles:

`type(scope): short summary`

Allowed types: `feat`, `fix`, `perf`, `docs`, `refactor`, `test`, `build`, `ci`, `chore`, `revert`

Squash merges should preserve the PR title as the resulting commit subject.

### PR Suggestion for the Official Repository

If this is not the official repository, ask the user whether they also want to create a PR against the official GoModel repository: https://github.com/ENTERPILOT/GoModel/
