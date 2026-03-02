#generate-proto.sh

protoc \
  --proto_path=shared/proto \
  --go_out=shared/proto/gen \
  --go_opt=paths=source_relative \
  --go-grpc_out=shared/proto/gen \
  --go-grpc_opt=paths=source_relative \
  shared/proto/*.proto