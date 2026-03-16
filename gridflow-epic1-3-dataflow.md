# GridFlow — Data Flow: Epic 1 to Epic 3

---

## Epic 1 — Shared Foundation

```
┌─────────────────────────────────────────────────────────────┐
│                     Shared Modules                          │
│                   (gridflow-shared)                         │
│                                                             │
│  config/         — loads .env, validates required fields    │
│  logger/         — zerolog structured logger, service_name  │
│  middleware/     — RequestLogger, JWTMiddleware             │
│  cache/          — Redis client, JWTCache                   │
│  proto/gen/      — generated protobuf types                 │
│                    VehiclePing, TelemetryAck,               │
│                    RouteCommand, IngestionService           │
└─────────────────────────────────────────────────────────────┘
         │                          │
         │ imported by              │ imported by
         ▼                          ▼
    Auth Service             Ingestion Service
```

---

## Epic 2 — Auth Service

```
                            Client (REST)
                                  │
                                  │ HTTP
                                  ▼
┌──────────────────────────────────────────────────────────────┐
│                       Auth Service                           │
│                      (services/auth)                         │
│                                                              │
│  POST /auth/register                                         │
│       │                                                      │
│       ▼                                                      │
│  AuthHandler.Register()                                      │
│    • binding validation (email, min password length)         │
│       │                                                      │
│       ▼                                                      │
│  AuthService.Register()                                      │
│    • UserRepo.UserExistsByEmail() ──────────────► PostgreSQL │
│    • 409 if email taken             ◄────────────            │
│    • bcrypt.GenerateFromPassword()                           │
│      (cost factor 12)                                        │
│    • UserRepo.CreateUser() ─────────────────────► PostgreSQL │
│    • return RegisterResponse                                 │
│                                                              │
│  POST /auth/login                                            │
│       │                                                      │
│       ▼                                                      │
│  AuthService.Login()                                         │
│    • UserRepo.GetUserByEmail() ─────────────────► PostgreSQL │
│    • bcrypt.CompareHashAndPassword()                         │
│    • TokenService.GenerateAccessToken()                      │
│      RS256, claims: user_id + role                           │
│      exp: 15 minutes                                         │
│    • TokenService.GenerateRefreshToken()                     │
│      32 random bytes, hex encoded                            │
│    • TokenService.HashToken() — SHA-256                      │
│    • TokenRepo.StoreRefreshToken() ─────────────► PostgreSQL │
│      stores hash only, never raw token                       │
│    • return { access_token, refresh_token }                  │
│                                                              │
│  POST /auth/refresh                                          │
│       │                                                      │
│       ▼                                                      │
│  AuthService.Refresh()                                       │
│    • HashToken(submitted_token)                              │
│    • TokenRepo.GetRefreshToken(hash) ───────────► PostgreSQL │
│    • check revoked flag                                      │
│    • check expiry (7 days)                                   │
│    • TokenRepo.RevokeRefreshToken() ────────────► PostgreSQL │
│      old token immediately invalidated                       │
│    • UserRepo.GetUserByID() ────────────────────► PostgreSQL │
│    • Generate new access + refresh tokens                    │
│    • Store new refresh token hash                            │
│    • return { new_access_token, new_refresh_token }          │
│                                                              │
│  Protected Routes                                            │
│  GET /...                                                    │
│       │                                                      │
│       ▼                                                      │
│  JWTMiddleware(publicKey)                                    │
│    • Extract Bearer token                                    │
│    • hashToken(tokenString) — SHA-256                        │
│    • JWTCache.Get(hash) ────────────────────────► Redis      │
│        Cache HIT ◄──────────────────────────────             │
│          inject user_id + role into context                  │
│          skip RSA verification                               │
│        Cache MISS                                            │
│          jwt.ParseWithClaims() — RS256 verify                │
│          extract user_id + role                              │
│          JWTCache.Set(hash, claims, ttl) ───────► Redis      │
│          inject into context                                 │
│    • c.Next()                                                │
│                                                              │
│  Database Schema                                             │
│  ┌────────────────┐    ┌──────────────────────┐              │
│  │    users       │    │   refresh_tokens     │              │
│  │────────────────│    │──────────────────────│              │
│  │ id             │    │ id                   │              │
│  │ email          │    │ user_id (FK)         │              │
│  │ password_hash  │    │ token_hash           │              │
│  │ role           │    │ revoked              │              │
│  │ created_at     │    │ expires_at           │              │
│  └────────────────┘    │ created_at           │              │
│                        └──────────────────────┘              │
└──────────────────────────────────────────────────────────────┘
```

---

## Epic 3 — Ingestion Service

```
                                        Vehicle Client
                                             │
                                             │  gRPC Bidirectional Stream
                                             │  StreamTelemetry
                                             ▼
┌───────────────────────────────────────────────────────────────────────────────────────────┐                          
│                                   Ingestion Service                                       │
│                                  (services/ingestion)                                     │
│                                                                                           │
│  StreamTelemetry Handler                                                                  │
│       │                                                                                   │
│       ▼                                                                                   │
│  1. Recv() ping                                                                           │
│       │                                                                                   │
│       ▼                                                                                   │
│  2. validatePing()                                                                        │
│     • vehicle_id not empty                                                                │
│     • lat ∈ [-90, 90]                                                                     │
│     • lng ∈ [-180, 180]                                                                   │
│     • timestamp not in future                                                             │
│       │                                                                                   │
│       ├── fail → TelemetryAck{INVALID} → client                                           │
│       │          return nil                                                               │
│       ▼                                                                                   │
│  3. RateLimiter.Allow(vehicle_id) ──────────────► Redis sorted set ratelimit:vehicle:<id> │
│     Lua script (atomic EVAL):  ◄───────────────────────────────────┘                      │
│     • ZREMRANGEBYSCORE — purge expired                                                    │
│     • ZCARD — count window entries                                                        │
│     • if count >= limit → return 0                                                        │
│     • ZADD score=now member=uuid                                                          │
│     • EXPIRE key ttl_seconds                                                              │
│       │                                                                                   │
│       ├── Redis error → log warn → allow by default                                       │
│       ├── rejected → TelemetryAck{RATE_LIMITED} → client                                  │
│       │              return nil (first ping)                                              │
│       │              continue (loop pings)                                                │
│       ▼                                                                                   │
│  4. SessionStore.Add(vehicle_id, stream)                                                  │
│     defer SessionStore.Remove(vehicle_id)                                                 │
│     in-memory concurrent map (sync.RWMutex)                                               │
│       │                                                                                   │
│       ▼                                                                                   │
│  5. Non-blocking channel push                                                             │
│     select:                                                                               │
│       case pings <- ping:                                                                 │
│         TelemetryAck{OK} → client                                                         │
│       default (channel full):                                                             │
│         TelemetryAck{RATE_LIMITED} → client                                               │
│                                                                                           │
│  ┌───────────────────────────────────────────────────────┐                                │
│  │              Worker Pool (10 goroutines)              │                                │
│  │                                                       │                                │
│  │  each goroutine:                                      │                                │
│  │  select {                                             │                                │
│  │    case ping <- pings:  → process(ping)               │                                │
│  │    case <-ctx.Done():   → drain channel → exit        │                                │
│  │  }                                                    │                                │
│  └───────────────────────────────────────────────────────┘                                │
│                     │                                                                     │
│                     ▼                                                                     │
│  KafkaPublisher.Publish(ping)                                                             │
│    • json.Marshal(ping)                                                                   │
│    • kafka.Message{                                                                       │
│        Key:   []byte(vehicle_id),                                                         │
│        Value: jsonBytes,                                                                  │
│      }                                                                                    │
│    • writer.WriteMessages() — synchronous                                                 │
│      RequiredAcks: RequireAll                                                             │
│      Balancer: Hash (key-based partition routing)                                         │
│       │                                                                                   │
│       ├── success → continue                                                              │
│       └── failure → log error                                                             │
│                     increment                                                             │
│                     telemetry_publish_errors_total                                        │
└───────────────────────────────────────────────────────────────────────────────────────────┘
                                             │
                                             │  kafka-go WriteMessages
                                             ▼
                                ┌─────────────────────────────┐
                                │           Kafka             │
                                │                             │
                                │   vehicle.telemetry topic   │
                                │                             │
                                │   vehicle_id → partition    │
                                │   (all pings from same      │
                                │   vehicle ordered on        │
                                │   same partition)           │
                                └─────────────────────────────┘
```

---

## Graceful Shutdown Sequence

```
SIGTERM / SIGINT received
     │
     ▼
cancel() ──────────────────► Worker pool stops accepting new pings
     │                        Workers drain remaining channel items
     │                        Workers call wg.Done()
     ▼
GracefulStop() ─────────────► No new streams accepted
     │                        Active streams finish naturally
     ▼
workerPool.Wait() ──────────► Block until all in-flight workers finish
     │
     ▼
kafkaPublisher.Close() ─────► Flush and close Kafka writer
     │
     ▼
Process exits
```

---

## How Epic 2 and Epic 3 Relate

```
Vehicle Client ──── gRPC ────► Ingestion Service
                                      │
                                      │ Does NOT validate JWT
                                      │ (vehicles use device keys,
                                      │  not user JWTs)

Human Client ───── HTTP ────► Auth Service
  (driver app,                        │
   dispatcher)              issues JWT access token
                                      │
                             Other services (Epic 4+)
                             use JWTMiddleware to validate
                             tokens on protected routes
```
---
