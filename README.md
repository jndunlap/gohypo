# GoHypo

A math-first statistical discovery engine built with Hexagonal Architecture and DRY principles.

## Architecture Overview

GoHypo implements a **Layered Pipeline Architecture** for statistical hypothesis generation and validation:

- **Layer 0**: Exhaustive relationship mapping (measurements, not truth)
- **Layer 1**: LLM generates hypotheses from Layer 0 artifacts
- **Validation**: Programmatic referee judges hypotheses through statistical rigor
- **Compounding**: Growing knowledge base of validated insights

## Key DRY Principles

1. **Single Canonical Input**: `domain/dataset/MatrixBundle` flows through all computation
2. **Universal Variable Resolution**: 4 generic as-of modes, not custom SQL per variable
3. **Stage Pipeline Pattern**: Rigor profiles = stage configurations
4. **Artifact-Only Persistence**: UI queries artifacts by kind, not custom endpoints
5. **Deterministic Replay**: Hash-based fingerprinting ensures reproducibility

## Project Structure

```
gohypo/
├── cmd/                    # Application entrypoints
│   ├── api/               # HTTP API server
│   │   └── main.go
│   ├── cli/               # CLI tools (sweep, battery, resolve)
│   │   └── main.go
│   ├── dev/               # Development tools (seed, smoke tests)
│   │   └── main.go
│   └── main.go            # Root command entrypoint
├── domain/                # Pure business logic (no external deps)
│   ├── core/             # Centralized types (ID, Time, Errors, Hash)
│   │   ├── errors.go
│   │   ├── hash.go
│   │   ├── id.go
│   │   └── time.go
│   ├── datareadiness/    # Data readiness pipeline
│   │   ├── ingestion/    # Data ingestion types
│   │   │   └── types.go
│   │   ├── profiling/    # Data profiling types
│   │   │   └── types.go
│   │   └── resolution/   # Data resolution orchestration
│   │       ├── orchestrator.go
│   │       └── readiness.go
│   ├── dataset/          # MatrixBundle (canonical data object)
│   │   ├── bundle.go
│   │   └── manifest.go
│   ├── stage/            # Pipeline execution types
│   │   └── types.go
│   ├── contracts/        # Variable contracts + compilation
│   │   ├── compile.go
│   │   └── types.go
│   ├── hypothesis/       # Hypothesis generation types
│   │   └── types.go
│   ├── run/              # Run execution types
│   │   └── types.go
│   ├── snapshot/         # Snapshot types
│   │   └── types.go
│   ├── stats/            # Statistical types
│   │   └── types.go
│   └── verdict/          # Validation result types
│       └── types.go
├── app/                   # Use case orchestration
│   ├── build_snapshot_service.go
│   ├── hypothesis_service.go
│   ├── matrix_resolver_service.go
│   ├── stage_runner.go
│   └── stats_sweep_service.go
├── ports/                 # Interface contracts
│   ├── battery.go        # Validation tests
│   ├── generator.go      # Hypothesis generation
│   ├── ledger.go         # Append-only artifact storage
│   ├── matrix_resolver.go # Variable-to-matrix resolution
│   ├── reader.go         # Read-only UI/API access
│   ├── registry.go       # Variable contract management
│   ├── rng.go            # Deterministic randomness
│   ├── snapshot.go       # Snapshot operations
│   └── stats.go          # Statistical computation
├── adapters/              # External system implementations
│   ├── battery/          # Validation test adapters
│   │   ├── confounder_stress_adapter.go
│   │   └── phantom_adapter.go
│   ├── datareadiness/    # Data readiness adapters
│   │   ├── coercer/
│   │   │   └── coercer.go
│   │   └── synthesizer/
│   │       └── synthesizer.go
│   ├── db/
│   │   └── postgres/     # PostgreSQL adapters
│   │       ├── ledger_adapter.go
│   │       ├── matrix_resolver_adapter.go
│   │       ├── migrations/  # Database schema
│   │       │   ├── 001_initial_schema.sql
│   │       │   ├── 002_test_data.sql
│   │       │   ├── 003_registry_versioning.sql
│   │       │   └── migrate.go
│   │       ├── registry_adapter.go
│   │       └── snapshot_adapter.go
│   ├── llm/              # LLM hypothesis generation
│   │   ├── generator_adapter.go
│   │   └── heuristic/
│   │       └── generator.go
│   └── stats/            # Statistical computation adapters
│       ├── engine/       # Consolidated statistical engine
│       │   ├── engine.go
│       │   ├── pairwise.go
│       │   ├── permutation.go
│       │   ├── relationship.go
│       │   └── stability.go
│       ├── pairwise_adapter.go
│       ├── permutation_adapter.go
│       └── stability_adapter.go
├── api/                   # HTTP API layer
│   ├── handlers.go       # Request handlers
│   └── server.go         # HTTP server
├── ui/                    # User interface
│   └── app.go            # UI application
├── internal/              # Internal packages
│   └── testkit/          # Testing utilities
│       ├── kit.go
│       ├── readiness.go
│       └── rng_adapter.go
├── gohypo/                # Legacy/submodule structure
│   ├── adapters/
│   │   └── db/
│   │       └── postgres/
│   │           └── matrix_resolver_adapter.go
│   ├── cmd/
│   │   ├── cli/
│   │   │   └── main.go
│   │   └── dev/
│   │       └── main.go
│   ├── domain/
│   │   └── dataset/
│   │       └── manifest.go
│   └── ports/
│       └── reader.go
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── test_generator         # Test generation utility
└── README.md              # This file
```

## Core Data Flow

1. **Input**: Variable contracts + snapshot specification
2. **Resolution**: Contracts compiled → MatrixBundle created via generic as-of resolvers
3. **Analysis**: MatrixBundle fed to stage pipeline (stats → validation → artifacts)
4. **Storage**: Artifacts written to append-only ledger
5. **Replay**: Same fingerprint → identical results

## Development Workflow

```bash
# Generate seed data
go run cmd/dev/main.go seed

# Run smoke tests
go run cmd/dev/main.go smoke

# CLI operations
go run cmd/cli/main.go sweep <snapshot-id>
go run cmd/cli/main.go battery <hypothesis-id>

# Start API server
go run cmd/api/main.go

# Test determinism
go run cmd/dev/main.go determinism <run-id>
```

## Key Invariants

- **No variable-specific code** outside tests
- **Everything resolves to MatrixBundle**
- **All computation is deterministic + replayable**
- **Domain logic never touches storage**
- **New features = new artifacts or stages, not new ports**
