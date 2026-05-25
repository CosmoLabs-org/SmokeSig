---
id: IDEA-035
legacy_id: IDEA-MO1X1P5F
title: Optional gRPC module via build tag
created: "2026-04-16T17:12:38.931104-03:00"
status: harvested
source: human
origin:
    session: 2026
promoted_to: ROAD-030
---

# Optional gRPC module via build tag

# Optional gRPC module via build tag

v0.3.0 added google.golang.org/grpc as a direct dep purely for the grpc_health assertion. This adds significant bloat to the binary. Consider splitting grpc_health into a build-tag gated file (e.g. //go:build grpc) or a separate module so users who don't need gRPC don't carry the dep. Same consideration for future heavy deps (kafka, nats, etc.).
