#!/bin/bash
mkdir -p coverage
# shellcheck disable=SC2034
export TEMPLATE_DIR=$(pwd)/templates
go test -v -race -covermode=atomic -coverprofile=coverage/coverage.out \
 -coverpkg=github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller,github.com/SENERGY-Platform/timescale-rule-manager/pkg/database,github.com/SENERGY-Platform/timescale-rule-manager/pkg/kafka,github.com/SENERGY-Platform/timescale-rule-manager/pkg/model,github.com/SENERGY-Platform/timescale-rule-manager/pkg/security,github.com/SENERGY-Platform/timescale-rule-manager/pkg \
  ./...
TEST_RESULT=$?
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
exit $TEST_RESULT
