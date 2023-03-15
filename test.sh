#!/bin/bash
docker-compose down # ensure reset
docker-compose up -d --wait
mkdir -p coverage
go test -race -covermode=atomic -coverprofile=coverage/coverage.out \
 -coverpkg=github.com/senergy-platform/timescale-rule-manager/pkg/controller,github.com/senergy-platform/timescale-rule-manager/pkg/database,github.com/senergy-platform/timescale-rule-manager/pkg/kafka,github.com/senergy-platform/timescale-rule-manager/pkg/model,github.com/senergy-platform/timescale-rule-manager/pkg/security,github.com/senergy-platform/timescale-rule-manager/pkg \
  ./...
TEST_RESULT=$?
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
docker-compose down # cleanup
exit $TEST_RESULT
