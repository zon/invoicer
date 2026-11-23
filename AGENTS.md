## Before a Pull Request

Bump the app version before you submit a pull request. Judge the SemVer component from the change, as [Release](docs/design/release.md) describes. Set `VERSION` and the chart `appVersion` to the new version, and bump the chart `version` in `deploy/chart/Chart.yaml`.

## Zpecs

* [Milestones](docs/milestones.md) - High level objectives
* [Regressions](regressions.md) - How to organize and resolve bugs and problems found during other work
* [Code](docs/zpecs/code.md) - Read before writing code
  * [Architecture](docs/zpecs/architecture.md) - Read before planning code structure
  * [Dependencies](docs/zpecs/dependencies.md) - Read before adding, changing, or updating code dependencies
  * [Containers](docs/zpecs/containers.md) - Read before building or delivering a container image
* [Prose](docs/zpecs/prose.md) - Read before writing any kind of prose
* [Prompts](docs/zpecs/prompts.md) - Read before writing an agent prompt
* [Spec](docs/zpecs/specs.md) - Read before writing a spec
* [Project](docs/zpecs/project.md) - Read before writing a project
* [Glossary](docs/zpecs/glossary.md) - Read when a term is unclear
* [Process](docs/zpecs/process.md) - Read before planning milestones, projects, or regressions
* [Design](docs/zpecs/design.md) - Read before writing a high level design doc
