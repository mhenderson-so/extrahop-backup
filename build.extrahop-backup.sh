#!/bin/sh
#
# builds patcher-client for linux
# used in teamcity mk_rpm_fpm

go run ./build.go -output ./cmd/extrahop-backup/
