#!/bin/bash

pwd=`pwd`

shopt -s extglob
cd $pwd/go && rm -rf api doc include lib misc test AUTHORS CONTRIBUTORS favicon.ico LICENSE PATENTS README robots.txt
cd $pwd/go/bin && rm !(go)
cd $pwd/go/pkg && rm !(*/runtime.a)
