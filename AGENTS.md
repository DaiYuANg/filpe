# Filpe AGENTS.md

## Project Overview

Filpe is a stateless file-processing runtime.

It is not a business platform and not a workflow engine.
Its responsibility is limited to:

- accepting file-processing jobs
- resolving input sources
- dispatching jobs to workers
- executing processors
- returning structured results and/or output artifacts

Filpe must remain:

- stateless
- horizontally scalable
- processor-driven
- middleware-oriented
- easy to operate
- simple in architecture

## Core Architecture Principles

### 1. Stateless first
Filpe must not depend on local persistent state.

Allowed:
- temporary local files during processing
- in-memory fallback backend for local development only

Not allowed:
- local persistent business state
- local durable job state
- assumptions that a request or job will hit the same instance twice

### 2. Clear separation of concerns
The codebase must keep these concerns separated:

- API layer
- backend / queue abstraction
- worker runtime
- source adapters
- processor implementations
- artifact handling
- models / schemas
- configuration

Do not mix these responsibilities in a single module unless the code is trivial.

### 3. Processor-oriented design
Processors are the core extension point.

Every file operation must be implemented as a processor.

Examples:
- excel.read
- excel.write
- image.resize
- image.convert
- pdf.extract_text

Do not hardcode processor-specific logic inside API handlers or queue backends.

### 4. Source abstraction is mandatory
Input sources must be abstracted.

Supported source types may include:
- upload
- object_storage
- url
- inline

Processors must not care where the file came from.
Before entering a processor, every input must be normalized into a staged input abstraction.

### 5. Result and artifact separation
Structured result data and generated file artifacts are different things.

Always model them separately.

- result: structured JSON-like output
- artifacts: generated files and their metadata

Do not overload one concept to represent both.

### 6. Backend pluggability
The system must support multiple backend modes.

At minimum:
- memory backend for local/single-process development
- rq/valkey backend for distributed execution

Do not tightly couple application logic to RQ or Valkey APIs.
Always go through an internal abstraction layer.

### 7. Simplicity over premature abstraction
Prefer straightforward code over speculative extensibility.

Allowed:
- small, focused abstractions
- explicit processor registration
- simple factories

Avoid:
- overly generic class hierarchies
- plugin systems that are not yet needed
- metaprogramming-heavy designs
- heavyweight or framework-dominating dependency injection patterns

## Project Boundaries

### Filpe should do
- accept processing jobs
- validate job requests
- stage input files
- run processors
- persist short-lived job state through configured backend
- expose job status
- return results and artifacts

### Filpe should NOT do
- user/account management
- RBAC/permission system
- business workflow orchestration
- domain-specific validation rules
- approval flow logic
- long-term analytics/reporting database
- becoming a general BPM engine

If a requested feature pushes Filpe toward business logic, reject that direction and keep the system focused.

## Code Organization Rules

Recommended package layout:

- `src/filpe/api/` — HTTP API layer
- `src/filpe/core/` — config, backend abstraction, runtime primitives
- `src/filpe/models/` — request/result/job schemas
- `src/filpe/sources/` — source adapters
- `src/filpe/processors/` — processor implementations
- `src/filpe/artifacts/` — artifact output handling
- `src/filpe/workers/` — worker startup and worker loop
- `src/filpe/utils/` — small reusable helpers only

Rules:
- API handlers must remain thin.
- Business-like processing logic must not live in route files.
- Processors must not directly read HTTP request objects.
- Backends must not know processor internals.
- Models must be explicit and strongly typed.
- Dependency wiring should happen near entry points, not across the whole codebase.

## Python and Style Rules

### Python version
Use Python 3.14 (see .python-version).

### Typing
Type hints are mandatory for public functions and non-trivial internal functions.

Avoid untyped code unless truly trivial.

### Data modeling
Use Pydantic models for:
- API request/response models
- job descriptors
- result payloads
- artifact metadata
- configuration where appropriate

Do not pass around raw `dict[str, Any]` when a clear model should exist.

## Dependency Injection Rules

### DI approach
Filpe uses Injector as a lightweight dependency injection library.

Injector is allowed only as a wiring tool for application assembly.
It must not become the center of the architecture.

### Allowed usage
Injector may be used for:
- application bootstrap
- wiring configuration objects
- wiring backend implementations
- wiring processor registries
- wiring service objects
- wiring worker runtime objects

Typical composition roots are:
- `src/filpe/main.py`
- `src/filpe/api/app.py`
- `src/filpe/workers/worker.py`

### Preferred style
Prefer explicit constructor injection.

Good:
- instantiate services through Injector modules/providers
- inject dependencies into service constructors
- keep dependency graphs shallow and understandable

Avoid:
- pulling dependencies from the container deep inside business code
- treating Injector as a global service locator
- hiding control flow behind excessive container magic
- injecting trivial helpers that are clearer as direct values or functions

### Container boundaries
The Injector container should remain near application entry points.

Core modules such as:
- processors
- source adapters
- artifact handlers
- backend implementations

must not depend directly on Injector APIs unless there is a strong reason.

These modules should primarily depend on normal constructor parameters and explicit interfaces/protocols.

### Testing
Tests should be able to instantiate important objects without requiring the full production container.

Prefer:
- direct construction in unit tests
- small testing modules/providers when needed

Avoid:
- making all tests depend on a large application container
- container-heavy test setup for simple cases

### Design rule
If Injector makes a change harder to read, harder to debug, or more magical, do not use Injector for that change.

### Function design
Prefer small functions with explicit inputs/outputs.

Avoid giant functions that mix:
- validation
- IO
- staging
- processing
- serialization
- response generation

### Class design
Use classes only when they provide real structure.

Prefer:
- simple service objects
- processor classes with explicit interfaces
- backend implementations behind a small protocol/interface
- explicit constructor dependencies

Avoid:
- deep inheritance trees
- framework-style magic base classes
- unnecessary patterns copied from Java enterprise code
- container-driven class design where object construction rules dominate the model

### Error handling
Errors must be explicit and meaningful.

Use domain-appropriate exceptions for:
- invalid request
- unsupported processor
- unsupported source
- staging failure
- processing failure
- backend failure

Do not swallow exceptions silently.
Do not return inconsistent error shapes.

### Logging
Use structured logging.

Logs should include:
- job_id
- processor name
- source type
- execution duration
- status
- failure reason where relevant

Do not use noisy ad-hoc print statements.

## API Design Rules

### API style
Use clear, predictable REST-style endpoints.

Suggested patterns:
- `POST /jobs`
- `POST /jobs:upload`
- `GET /jobs/{job_id}`
- `GET /jobs/{job_id}/result`
- `GET /jobs/{job_id}/artifacts`

### API constraints
- Keep handlers thin
- Validate everything at boundaries
- Return stable response models
- Do not leak backend-specific details such as raw RQ job internals

### Sync vs async
Support both only where justified.

Rules:
- light processors may support sync execution
- heavy or long-running processors must run async
- sync execution must never secretly behave like distributed async without telling the caller

## Worker Rules

### Worker role
Workers execute jobs only.
Workers must not expose business-facing HTTP APIs.

### Worker behavior
Workers must:
- load configuration
- initialize backend connectivity
- register processors
- fetch jobs
- resolve source
- stage files
- execute processor
- write result/artifacts
- update status

### Worker isolation
Different processor categories may run in different worker pools.

Examples:
- excel workers
- image workers
- pdf workers
- ocr workers

Do not assume all processors have the same resource profile.

## File Handling Rules

### Temporary files
All temporary files must be created under a managed temp directory.

They must be cleaned up after processing whenever possible.

### File safety
Never trust file extension alone.

Validate at least where practical:
- source type
- file size
- content type
- processor compatibility

Large/untrusted files must be treated carefully.

### Artifact handling
Artifacts must be represented with explicit metadata, such as:
- name
- media type
- size
- location
- checksum if applicable

## Testing Rules

### Required tests
At minimum, include tests for:
- configuration loading
- backend selection
- job submission flow
- source staging
- processor registry
- one happy-path processor execution
- one error-path processor execution

### Testing style
Prefer focused tests over huge integration-only suites.

Good:
- unit tests for source adapters
- unit tests for processors
- integration tests for job submission and execution

Avoid:
- fragile tests tightly coupled to internal implementation details

## Dependency Rules

### Keep dependencies minimal
Only add a dependency when it clearly reduces complexity or provides significant value.

Before adding a package, check:
- maintenance status
- ecosystem maturity
- necessity
- whether the feature is small enough to implement locally

### Avoid unnecessary frameworks
Do not introduce:
- heavyweight DI frameworks
- ORM systems
- background task frameworks beyond the chosen backend model
- broad utility libraries that encourage sloppy coding

## Performance Rules

### Performance posture
Optimize for:
- predictable execution
- bounded resource usage
- horizontal scaling
- simple operational behavior

Do not micro-optimize prematurely.

### Important considerations
Be mindful of:
- large file memory usage
- temp file cleanup
- streaming where possible
- processor-specific resource constraints

## Documentation Rules

Any non-trivial addition must update documentation where applicable.

At minimum, keep aligned:
- README
- processor registration docs
- config reference
- API examples if endpoints change

## Change Rules for AI Assistants

When making changes:

1. Preserve architecture boundaries.
2. Do not move business logic into infrastructure layers.
3. Do not bypass internal abstractions just for convenience.
4. Prefer explicit models over loose dictionaries.
5. Prefer small, reviewable changes.
6. If a change introduces new concepts, document them.
7. If a change conflicts with these rules, follow these rules.
8. Use Injector only for lightweight wiring near entry points, not as a pervasive service locator.

## What AI assistants should avoid

Do not:
- rewrite the whole project without need
- introduce speculative abstractions
- add hidden magic
- couple processors to HTTP or backend internals
- add database-oriented assumptions
- turn Filpe into a workflow engine
- add broad "manager" classes with unclear responsibilities
- use dynamic tricks where direct code is clearer
- spread Injector usage into every layer of the project
- resolve dependencies from the container inside processors, source adapters, or core runtime code unless truly necessary

## Preferred implementation mindset

When in doubt, choose the option that is:

- simpler
- more explicit
- easier to debug
- easier to operate
- more aligned with stateless middleware design
- safer for future horizontal scaling
