#!/bin/bash

# import go code from $GOROOT into current directory

mkdir go
SANDBOXROOT=./go

cp -r $GOROOT/bin $SANDBOXROOT/
cp -r $GOROOT/pkg $SANDBOXROOT/
cp -r $GOROOT/src $SANDBOXROOT/
cp $GOROOT/VERSION $SANDBOXROOT/
