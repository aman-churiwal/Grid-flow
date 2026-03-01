# create-topics.sh

docker exec kafka kafka-topics.sh \
  --create \
  --if-not-exists \
  --bootstrap-server localhost:9092 \
  --topic vehicle.telemetry \
  --partitions 12 \
  --replication-factor 1 \
  --config retention.ms=604800000

docker exec kafka kafka-topics.sh \
  --create \
  --if-not-exists \
  --bootstrap-server localhost:9092 \
  --topic anomaly.detected \
  --partitions 6 \
  --replication-factor 1 \
  --config retention.ms=1209600000

docker exec kafka kafka-topics.sh \
  --create \
  --if-not-exists \
  --bootstrap-server localhost:9092 \
  --topic route.command \
  --partitions 6 \
  --replication-factor 1 \
  --config retention.ms=259200000

docker exec kafka kafka-topics.sh \
  --create \
  --if-not-exists \
  --bootstrap-server localhost:9092 \
  --topic audit.log \
  --partitions 3 \
  --replication-factor 1 \
  --config retention.ms=-1

docker exec kafka kafka-topics.sh \
  --create \
  --if-not-exists \
  --bootstrap-server localhost:9092 \
  --topic notification.outbox \
  --partitions 6 \
  --replication-factor 1 \
  --config retention.ms=604800000

docker exec kafka kafka-topics.sh \
  --list \
  --bootstrap-server localhost:9092