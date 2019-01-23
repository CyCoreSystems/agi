#!/bin/bash
dep ensure
write_mailmap > CONTRIBUTORS
go build ./
golangci-lint run ./...
