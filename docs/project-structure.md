# Approximate project structure

This is an approximate layout, not a mandatory template.

```text
cmd/gc/               Application startup
docs/                 Project documentation
framework/            Reusable across projects
  journal/            In-memory fact journal
  spec/               Scenario runner and HTTP, HTML, and fact assertions
internal/             Application implementation
  agent/              Agent runs: clone, Codex subprocess, commit, and push
  domain/             Business logic
  github/             GitHub API client
  web/                HTTP handlers, templates, hourly wake-ups, and agent orchestration
specs/                Project requirements and executable scenarios
  contract/           Fact types and dependency interfaces
  extensions/         Project-specific scenario methods, fakes, and app wiring
ui/                   Shared interface platform: htmx, its configuration, page head, static files
```

The project uses the framework; the framework knows nothing about the project.

For a new project, reuse `framework`. For a rebuild, retain `framework`, `specs`, and `ui`,
and reimplement `internal` and `cmd/gc`.
