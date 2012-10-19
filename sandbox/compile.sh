#!/bin/bash

# This script shows how to compile/run code in the sandbox:
#
# 1) set GOROOT
# 2) run go compiler in sandbox

env - \
GOROOT=`pwd`/go \
GOPATH= \
./go/bin/go run helloworld.go
