#generate-proto.sh
# Run "bash scripts/generate-proto.sh" from root folder

protoc \
  --proto_path=shared/proto \
  --go_out=shared/proto/gen \
  --go_opt=paths=source_relative \
  --go-grpc_out=shared/proto/gen \
  --go-grpc_opt=paths=source_relative \
  shared/proto/*.proto