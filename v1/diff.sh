#!/bin/bash

for X in $(ls *_test.go); do diff -u ../../go/src/encoding/json/$X $X; done
