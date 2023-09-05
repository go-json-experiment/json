#!/bin/bash

go test | egrep -o "FAIL: Test[^ ]+" > failing.new
diff -u -U-1 failing.old failing.new && cat failing.new

if [[ "$1" == "-update" ]]; then
    cp failing.new failing.old
fi
