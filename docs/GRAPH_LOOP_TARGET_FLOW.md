# Graph + loop target flow

**Status:** Target diagram from the graph-loop audit. Canonical current behavior: [`GRAPH_AND_LOOPS.md`](GRAPH_AND_LOOPS.md).

Smallest evolution of the engine so the orchestrator is graph-driven and loop-driven. No new orchestration framework.

Updates since this note was written:

- Outer walk `Engine.RunGraph` / `prdpr phase` is **IMPLEMENTED** (sequential READY).
- READY-only selection (refuse BLOCKED/`--phase` override) is **IMPLEMENTED**.
- RUNNING persist on prepare is **IMPLEMENTED**.
- Completeness still **PARTIAL** (skipped on `llm.None`).
- Upstream rewind still **PARTIAL**.
- Runtime inner restart loop still **PARTIAL**.
- Parallel Ready() and task-level graphs remain **OUT OF SCOPE**.

**Status legend**

- **IMPLEMENTED** — already drives execution
- **PARTIAL** — package or persistence exists; scheduler does not fully own the step
- **FUTURE** — do not build yet unless dogfood proves the need

**Actors:** `[ENGINE]` `[GRAPH]` `[LLM]` `[CURSOR]` `[SHELL]` `[HUMAN]` `[GITHUB]` `[CI]`

```mermaid
flowchart TB
  classDef impl fill:#1b6b3a,color:#fff,stroke:#0d3d21
  classDef part fill:#8a6d12,color:#fff,stroke:#5a4508
  classDef fut fill:#555,color:#fff,stroke:#333
  classDef human fill:#8a1f3a,color:#fff,stroke:#5a1024

  subgraph PRDGate["PRD Gate"]
    A["IMPLEMENTED [ENGINE] validate-prd / contractGate"]
    B["PARTIAL [LLM] completeness review<br/>only when adapter configured; persist human request"]
    C["IMPLEMENTED [HUMAN] REJECTED → edit PRD → re-validate"]
  end

  subgraph ProjectBootstrap["Project Bootstrap"]
    D["IMPLEMENTED [ENGINE] type select + Studio place"]
    E["IMPLEMENTED [HUMAN] ambiguous placement / unsafe adopt"]
    F["IMPLEMENTED [SHELL] git init + baseline"]
    G["PARTIAL [GITHUB] remote / ruleset when enabled"]
  end

  subgraph PhaseGraph["OUTER GRAPH — PRD phases"]
    H["IMPLEMENTED [GRAPH] build + persist graph.json"]
    I["IMPLEMENTED [GRAPH] select READY only<br/>refuse BLOCKED / non-READY --phase"]
    J["IMPLEMENTED [GRAPH] SetStatus RUNNING then persist"]
    K["IMPLEMENTED [ENGINE] RunGraph after COMPLETED:<br/>Refresh → next Ready() in same run"]
    L["FUTURE [GRAPH] parallel Ready() execution"]
    M["FUTURE task-level graph / live rewrite"]
  end

  subgraph ImplLoop["INNER LOOP — one phase"]
    N["IMPLEMENTED [ENGINE] prepare packet"]
    O["IMPLEMENTED [CURSOR] implement packet<br/>plugin session XOR headless worker — not both"]
    P["IMPLEMENTED [SHELL] independent verify"]
    Q["IMPLEMENTED [ENGINE] diagnose.Classify"]
    R["PARTIAL [LLM] review/diagnose when router selects a model"]
    S["IMPLEMENTED [ENGINE] bounded repair ≤ 3 product attempts"]
    T["IMPLEMENTED [CURSOR] bounded repair packet"]
    U["IMPLEMENTED [SHELL] verify again"]
    V["PARTIAL [GRAPH] rewind/replay when origin is upstream"]
  end

  subgraph Delivery["Delivery"]
    W["IMPLEMENTED [ENGINE] verified commit gate"]
    X["PARTIAL [GITHUB] milestone PR by pr_boundary"]
    Y["PARTIAL [CI] observe checks"]
    Z["IMPLEMENTED [ENGINE] merge refused unless AutoMergeEnabled"]
  end

  subgraph RuntimeLoop["RUNTIME LOOP"]
    AA["IMPLEMENTED [ENGINE] start after AllCompleted"]
    AB["IMPLEMENTED [SHELL] observe ready"]
    AC["PARTIAL [LLM] runtime diagnose"]
    AD["PARTIAL [CURSOR] runtime repair packet"]
    AE["PARTIAL [ENGINE] restart observe; infra does not consume attempts"]
  end

  subgraph HumanIntervention["HUMAN LOOP"]
    AF["IMPLEMENTED [ENGINE] one request → WAITING_FOR_HUMAN"]
    AG["IMPLEMENTED [HUMAN] feedback"]
    AH["PARTIAL [ENGINE] resume continues the owning loop<br/>not a dead VERIFICATION_FAILED dump"]
  end

  A --> B
  A -->|REJECTED| C --> A
  B --> D
  D --> E
  D --> F --> G --> H --> I --> J --> N --> O --> P
  P -->|fail| Q --> R
  R -->|repair| S --> T --> U --> P
  R -->|human| AF
  P -->|VERIFIED| W --> K
  K -->|READY remains| I
  K -->|AllCompleted| X --> Y --> Z --> AA --> AB
  AB -->|fail| AC --> AD --> AE --> AB
  AB -->|ready| DONE["IMPLEMENTED project complete"]
  AE -->|exhausted/infra| AF
  S -->|exhausted| AF
  AF --> AG --> AH
  AH --> I
  AH --> P
  AH --> AA
  I -.-> L
  H -.-> M
  Q --> V

  class A,C,D,E,F,H,I,J,K,N,O,P,Q,S,T,U,W,Z,AA,AB,AF,AG,DONE impl
  class B,G,R,V,X,Y,AC,AD,AE,AH part
  class L,M fut
```

## What to wire (smallest set)

Items 1–3 are **done** (`RunGraph` walks READY after COMPLETED). Remaining:

4. `repair.Rewind` + `applyRewind` exist; they need a real upstream origin from diagnose, plus Git checkout of the origin checkpoint — not a new graph store. **PARTIAL**
5. Completeness/placement should always write the same human request files. **PARTIAL** for completeness when LLM is None (skipped)
6. Runtime already has incident + `MaxAttempts`; close the loop by invoking the same repair worker bound as `RunPhase`, without a second scheduler type. **PARTIAL**

## Explicitly out of scope until proven

- Task-level DAG
- Dynamic graph mutation / autonomous rewrite
- Parallel `Ready()` workers
- Subagent spawning (`Decide` stays advisory)
- New graph persistence format
- Multi-agent orchestration
