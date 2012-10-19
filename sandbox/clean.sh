#!/bin/bash

pwd=`pwd`

shopt -s extglob
cd $pwd/go && rm -rf api doc include lib misc test favicon.ico robots.txt
cd $pwd/go/bin && rm -f !(go)

# pkg dir
cd $pwd/go/pkg && rm -rf obj
for dir in $pwd/go/pkg/*/
do
	b=`basename $dir`
	if [ ! "$b" = "tool" ]; then
		cd $dir && rm -rf !(runtime.a)
	fi
done
