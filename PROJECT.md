# Go Microservices E-Commerce Platform (GraphQL + gRPC)

> Master plan. We build step by step. Tick boxes as we finish. Do not add a resume bullet until the feature works.

---

## 1. Goal

Build and publish on GitHub an advanced backend project that proves software developer skills:

- **Go** for all services
- **GraphQL** (gqlgen) as the single client-facing API
- **gRPC + Protobuf** for all internal service-to-service calls
- Distributed-systems depth: saga, outbox, idempotency, retries, circuit breaker
- Production habits: tests, CI, tracing, metrics, load tests, docs

**Success =** a recruiter opens the repo, sees the architecture diagram, runs `docker compose up`, fires a GraphQL query, and an interviewer can dig into any line and get a confident answer.

---

## 2. Architecture

```
Client (GraphiQL / curl / k6)
        |
        v
 GraphQL Gateway (gqlgen, DataLoader, JWT check, rate limit)
        |  gRPC (interceptors: auth, tracing, logging, retry)
        +--------------+---------------+---------------+
        v              v               v               v
   Users/Auth      Products         Orders         Inventory
   (Postgres)      (Postgres        (Postgres      (Postgres)
                    + Redis)         + outbox)
                                       |
                              NATS JetStream (events)
                  order.created -> inventory.reserved / inventory.failed
                  -> order.confirmed / order.cancelled (compensation)
```

### Service responsibilities

| Service | Owns | Key RPCs |
|---|---|---|
| **Gateway** | GraphQL schema, auth check, DataLoaders | queries, mutations, (optional) subscription |
| **Users/Auth** | users, password hashes, JWT issue/verify | Register, Login, GetUser, GetUsersByIDs, ValidateToken |
| **Products** | catalog, prices | CreateProduct, GetProduct, GetProductsByIDs, ListProducts |
| **Orders** | orders, order items, saga state, outbox | CreateOrder, GetOrder, ListOrdersByUser |
| **Inventory** | stock levels, reservations | GetStock, Reserve, Release, Commit |

Payments: **mocked** inside the saga (random success/fail flag) to keep scope tight.

---

## 3. Tech Stack

| Concern | Choice |
|---|---|
| Language | Go 1.22+ |
| GraphQL | gqlgen |
| RPC | gRPC, protobuf, `buf` for lint/generate |
| DB | PostgreSQL (one DB per service), `pgx`, `sqlc` or plain SQL |
| Migrations | golang-migrate |
| Cache | Redis |
| Events | NATS JetStream (simpler than Kafka; mention Kafka as alternative in DESIGN.md) |
| Auth | JWT (access token), bcrypt/argon2 |
| Observability | OpenTelemetry, Jaeger, Prometheus, Grafana, structured logs (`slog`) |
| Testing | `testing`, testify, testcontainers-go |
| Load test | k6 (GraphQL), ghz (gRPC) |
| Infra | Docker, Docker Compose, GitHub Actions; optional Kubernetes (kind) |

---

## 4. Repo Structure

```
ecommerce-platform/
├── README.md
├── DESIGN.md
├── PROJECT.md
├── Makefile
├── docker-compose.yml
├── buf.yaml / buf.gen.yaml
├── proto/
│   ├── users/v1/users.proto
│   ├── products/v1/products.proto
│   ├── orders/v1/orders.proto
│   ├── inventory/v1/inventory.proto
│   └── events/v1/events.proto
├── gen/                      # generated protobuf code
├── services/
│   ├── gateway/
│   │   ├── graph/ (schema.graphqls, resolvers, dataloaders)
│   │   ├── middleware/
│   │   └── cmd/main.go
│   ├── users/      (cmd, internal/{server,repo,auth}, migrations)
│   ├── products/   (cmd, internal/{server,repo,cache}, migrations)
│   ├── orders/     (cmd, internal/{server,repo,saga,outbox}, migrations)
│   └── inventory/  (cmd, internal/{server,repo,consumer}, migrations)
├── pkg/
│   ├── interceptors/   (auth, logging, tracing, recovery, retry)
│   ├── config/
│   ├── observability/  (otel, metrics, logger)
│   └── events/         (NATS publisher/consumer helpers)
├── deploy/
│   ├── prometheus.yml
│   ├── grafana/
│   └── k8s/ (optional)
├── loadtest/
│   ├── k6/
│   └── ghz/
└── .github/workflows/ci.yml
```

---

## 5. Data Model (starting point)

**users**: id (uuid), email (unique), password_hash, name, created_at
**products**: id, name, description, price_cents, currency, created_at
**inventory**: product_id, available, reserved, updated_at
**reservations**: id, order_id, product_id, qty, status (RESERVED/COMMITTED/RELEASED), created_at
**orders**: id, user_id, status (PENDING/CONFIRMED/CANCELLED), total_cents, idempotency_key (unique per user), created_at
**order_items**: id, order_id, product_id, qty, unit_price_cents
**outbox**: id, aggregate_id, event_type, payload (jsonb), created_at, published_at (nullable)
**processed_events** (inventory): event_id (pk), processed_at, for consumer idempotency

---

## 6. GraphQL Schema (draft)

```graphql
type User { id: ID!, email: String!, name: String!, orders: [Order!]! }
type Product { id: ID!, name: String!, description: String, priceCents: Int!, stock: Int! }
type OrderItem { product: Product!, quantity: Int!, unitPriceCents: Int! }
type Order { id: ID!, status: OrderStatus!, items: [OrderItem!]!, totalCents: Int!, createdAt: String! }
enum OrderStatus { PENDING CONFIRMED CANCELLED }

type AuthPayload { token: String!, user: User! }

type Query {
  me: User!
  product(id: ID!): Product
  products(limit: Int = 20, offset: Int = 0): [Product!]!
  order(id: ID!): Order
  myOrders: [Order!]!
}

type Mutation {
  register(email: String!, password: String!, name: String!): AuthPayload!
  login(email: String!, password: String!): AuthPayload!
  createProduct(name: String!, description: String, priceCents: Int!, initialStock: Int!): Product!
  placeOrder(items: [OrderItemInput!]!, idempotencyKey: String!): Order!
}

input OrderItemInput { productId: ID!, quantity: Int! }
```

---

## 7. The Hard Parts (what makes this advanced)

### 7.1 DataLoader (N+1 fix)
- Problem: `products { ... }` inside `orders { items { product } }` triggers one gRPC call per item.
- Fix: per-request DataLoader batching IDs into one `GetProductsByIDs` call.
- Proof: add a test/log counting gRPC calls before vs after. Put the numbers in README.

### 7.2 Saga with compensation (choreography)
1. `placeOrder` -> Orders saves order as PENDING + writes `order.created` to outbox (same DB tx).
2. Outbox relay publishes to NATS.
3. Inventory consumes `order.created`, tries to reserve all items atomically.
   - success -> publish `inventory.reserved`
   - failure -> publish `inventory.failed`
4. Orders consumes:
   - `inventory.reserved` -> (mock payment) -> CONFIRMED, publish `order.confirmed`
   - `inventory.failed` or payment failed -> CANCELLED, publish `order.cancelled`
5. Inventory consumes `order.cancelled` -> releases reservations (compensation).

Must-have tests: happy path, out-of-stock, payment failure triggers release, duplicate event delivery.

### 7.3 Outbox pattern
- Never publish to NATS directly inside the request handler.
- Write event row in the same transaction as the order; a background relay publishes and marks `published_at`.
- Guarantees at-least-once delivery without dual-write bugs.

### 7.4 Idempotency
- **API level:** `idempotency_key` on `placeOrder`; unique (user_id, key). Retry returns the same order.
- **Consumer level:** `processed_events` table; skip already-handled event IDs.

### 7.5 Resilience
- Per-call timeouts via `context`.
- Retry with exponential backoff + jitter, only for idempotent/safe calls.
- Circuit breaker (e.g. `sony/gobreaker`) around gateway -> service calls.
- Graceful shutdown (drain gRPC, stop consumers, flush outbox).

### 7.6 Auth propagation
- Gateway validates JWT, puts `user_id` in gRPC metadata.
- Service-side unary interceptor reads metadata into context; services authorize on it.
- Never trust client-supplied user IDs.

### 7.7 Observability
- OpenTelemetry trace from GraphQL request through gRPC calls and NATS events (propagate trace context in message headers).
- Prometheus metrics: request count, latency histogram, saga outcomes, outbox lag.
- Grafana dashboard JSON committed to repo.

### 7.8 Optional stretch (pick one)
- GraphQL query depth/complexity limits + persisted queries
- Kubernetes deployment on `kind` with manifests/Helm
- GraphQL subscription `orderStatusChanged` fed by NATS

---

## 8. Roadmap (weekends only, ~10-12 weekends)

### Phase 0: Setup (weekend 1)
- [ ] Install Go, Docker, buf, protoc plugins, gqlgen, migrate, k6, ghz
- [ ] Create GitHub repo, `go.mod` (single module or `go.work`), Makefile
- [ ] `docker-compose.yml` with Postgres(es), Redis, NATS
- [ ] Pick final repo name; initial README skeleton

### Phase 1: Foundations (weekends 2-3)
- [ ] Write `users.proto` and `products.proto`; set up `buf generate`
- [ ] Users service: Register, Login (bcrypt, JWT), GetUser, migrations
- [ ] Products service: CRUD + list, migrations
- [ ] Shared `pkg/` basics: config, slog logger, recovery + logging interceptors
- [ ] Gateway: gqlgen schema for register/login/products, calling gRPC
- [ ] Docker Compose runs all of it
- [ ] Unit tests for repos/handlers

### Phase 2: Core flow (weekends 4-5)
- [ ] Inventory service + proto (Reserve/Release/Commit with row-level locking)
- [ ] Orders service + proto; synchronous `placeOrder` first (no events yet)
- [ ] JWT auth interceptor + metadata propagation; `me`, `myOrders` queries
- [ ] DataLoaders for users/products in gateway
- [ ] Integration tests with testcontainers-go

### Phase 3: The hard parts (weekends 6-8)
- [ ] NATS JetStream wiring, `events.proto`
- [ ] Outbox table + relay in Orders
- [ ] Convert `placeOrder` to async saga (PENDING -> CONFIRMED/CANCELLED)
- [ ] Inventory consumer + compensation on cancel
- [ ] Consumer idempotency (`processed_events`) and API idempotency keys
- [ ] Retries/backoff, timeouts, circuit breaker
- [ ] Redis cache-aside for products (invalidate on update)
- [ ] Saga tests: happy, out-of-stock, payment fail, duplicate delivery

### Phase 4: Production polish (weekends 9-10)
- [ ] OpenTelemetry tracing end-to-end + Jaeger in Compose
- [ ] Prometheus metrics + Grafana dashboard
- [ ] k6 (GraphQL) and ghz (gRPC) load tests; record p50/p95/p99, req/s
- [ ] Before/after numbers for DataLoader and Redis cache
- [ ] GitHub Actions: lint (golangci-lint), `buf lint`, tests, build images
- [ ] Graceful shutdown everywhere

### Phase 5: Docs and resume (weekend 11-12)
- [ ] README: pitch, architecture diagram, quickstart (<= 3 commands), sample queries, load-test table
- [ ] DESIGN.md: trade-offs (see section 10)
- [ ] Architecture diagram (Mermaid or draw.io image)
- [ ] 2-minute demo GIF/video in README
- [ ] Optional stretch item from 7.8
- [ ] Final resume bullets with real numbers

---

## 9. Definition of Done

- [ ] `make up` (or `docker compose up`) starts the whole system
- [ ] A full order works end to end, and the failure path compensates correctly
- [ ] Tests pass in CI; badge in README
- [ ] Load-test numbers recorded in README
- [ ] Traces visible in Jaeger, dashboard in Grafana
- [ ] README and DESIGN.md complete
- [ ] No secrets in repo (`.env.example` only)

---

## 10. DESIGN.md Topics (interview material)

Write one short paragraph each, as we make the decision:

1. Why gRPC internally and GraphQL at the edge
2. Why choreography saga vs orchestration (and when you'd switch)
3. Why saga instead of 2PC
4. Why outbox instead of publishing directly
5. At-least-once delivery and why consumers must be idempotent
6. Database per service and what it costs (no cross-service joins)
7. NATS JetStream vs Kafka
8. Cache invalidation strategy for products
9. Where the circuit breaker sits and what it does on open
10. Known limitations and what you'd do next (sharding, multi-region, payment provider)

---

## 11. Interview Questions to Be Ready For

- How does your system behave if Inventory is down when an order is placed?
- What happens if the Orders service crashes after writing the order but before publishing the event?
- How do you stop a retried mutation from creating two orders?
- How did you solve N+1 in GraphQL, and how did you prove it?
- Why not just use REST everywhere?
- How do you trace one request across services and a message queue?
- What would break first at 10x load, and how would you find out?

---

## 12. Resume Bullets (fill real numbers only after built)

- Built a Go microservices e-commerce backend with a GraphQL (gqlgen) gateway and gRPC inter-service communication across 4 services.
- Implemented an event-driven order saga with outbox pattern, idempotent consumers, and compensating transactions on NATS JetStream.
- Reduced gateway backend calls by **X%** using DataLoader batching and cut product read latency by **Y%** with Redis cache-aside.
- Load-tested to **Z req/s** with p95 under **W ms** (k6/ghz); added OpenTelemetry tracing, Prometheus metrics, and CI via GitHub Actions.

---

## 13. Rules of Engagement

1. Small commits, meaningful messages (`feat(orders): add outbox relay`). GitHub history is part of the proof.
2. One phase at a time. Do not start Phase 3 with Phase 2 half-working.
3. Write the DESIGN.md paragraph the same day you make the decision.
4. Every feature gets at least one test.
5. If scope slips, cut stretch items, never the saga, tests, or docs.

---

## 14. Progress Log

| Date | Phase | Done | Next |
|---|---|---|---|
| | | | |

---

## 15. How We Work Together

Each session: tell me the phase/step. I give complete runnable code for that step, you run it, we fix errors, you commit, and we tick the box above.

**First step when ready:** Phase 0 setup, then `users.proto` and the Users service.
