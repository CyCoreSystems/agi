#!/bin/bash
write_mailmap > CONTRIBUTORS
go build ./
golangci-lint run ./...
