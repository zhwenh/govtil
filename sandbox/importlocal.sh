#!/bin/bash

# import go code from $GOROOT into current directory

mkdir go
SANDBOXROOT=./go

mkdir $SANDBOXROOT/bin
cp $GOROOT/bin/go $SANDBOXROOT/bin

mkdir $SANDBOXROOT/pkg
cp -r $GOROOT/pkg/tool $SANDBOXROOT/pkg

# runtimes
for dir in $GOROOT/pkg/*/
do
	r=`basename $dir`
	if [ "$r" = "tool" ]; then
		continue
	fi
	if [ "$r" = "obj" ]; then
		continue
	fi
	mkdir $SANDBOXROOT/pkg/$r
	cp $GOROOT/pkg/$r/runtime.a $SANDBOXROOT/pkg/$r
done

cp -r $GOROOT/src $SANDBOXROOT/
cp $GOROOT/VERSION $SANDBOXROOT/
