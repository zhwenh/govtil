#!/bin/bash

hg clone -r release -b default -u release https://code.google.com/p/go
cd go/src
. ./all.bash
