#!/bin/bash
dep ensure
write_mailmap > CONTRIBUTORS
go build ./
gometalinter --disable=gotype --vendor ./...
