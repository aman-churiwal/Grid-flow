[//]: # (kafka-topics.md)

| Topic Name            | Purpose                                           | Partitions | Replication Factor | Retention Policy              |
|-----------------------|--------------------------------------------------|------------|--------------------|------------------------------|
| `vehicle.telemetry`   | Raw GPS pings from vehicles                      | 12         | 1                  | 7 days (time-based)          |
| `anomaly.detected`    | Events flagged by AI or heuristic engine         | 6          | 1                  | 14 days (time-based)         |
| `route.command`       | Rerouting instructions for vehicles              | 6          | 1                  | 3 days (time-based)          |
| `audit.log`           | Append-only compliance and audit trail           | 3          | 1                  | Infinite retention (no TTL)  |
| `notification.outbox` | Events to notify drivers or dispatchers          | 6          | 1                  | 7 days (time-based)          |