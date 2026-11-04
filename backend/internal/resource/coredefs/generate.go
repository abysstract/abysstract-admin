package coredefs

// Test protobuf fixtures use an isolated Buf module and are regenerated in place.
//go:generate sh -c "cd ../../../.. && buf lint --config buf.resource-test.yaml --path backend/internal/resource/schema/testpb --path backend/internal/resource/schema/testenum"
//go:generate sh -c "cd ../../../.. && buf generate --config buf.resource-test.yaml --template buf.resource-test.gen.yaml --path backend/internal/resource/schema/testpb --path backend/internal/resource/schema/testenum"
