#!/bin/bash

if [ -d "go/.hg" ]; then
	hg -R go update --clean -r release
else
	hg clone -r release -b default -u release https://code.google.com/p/go
fi

cd go/src
. ./all.bash
