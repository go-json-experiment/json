#!/usr/bin/env bash

GOROOT=${1:-../go}
JSONROOT="."

/bin/rm -r $JSONROOT/*.go $JSONROOT/internal $JSONROOT/jsontext $JSONROOT/v1
cp -r $GOROOT/src/encoding/json/v2/*.go $JSONROOT/
cp -r $GOROOT/src/encoding/json/internal/ $JSONROOT/internal/
cp -r $GOROOT/src/encoding/json/jsontext/ $JSONROOT/jsontext/
mkdir $JSONROOT/v1
for X in $GOROOT/src/encoding/json/v2_*.go; do
    cp $X $JSONROOT/v1/$(basename $X | sed "s/v2_//")
done
cd $JSONROOT
for X in $(git ls-files --cached --others --exclude-standard | grep ".*[.]go$"); do
    if [ ! -e "$X" ]; then
        continue
    fi
    sed -i '/go:build goexperiment.jsonv2/d' $X
    sed -i 's|"encoding/json/v2"|"github.com/go-json-experiment/json"|' $X
    sed -i 's|"encoding/json/internal"|"github.com/go-json-experiment/json/internal"|' $X
    sed -i 's|"encoding/json/internal/jsonflags"|"github.com/go-json-experiment/json/internal/jsonflags"|' $X
    sed -i 's|"encoding/json/internal/jsonopts"|"github.com/go-json-experiment/json/internal/jsonopts"|' $X
    sed -i 's|"encoding/json/internal/jsontest"|"github.com/go-json-experiment/json/internal/jsontest"|' $X
    sed -i 's|"encoding/json/internal/jsonwire"|"github.com/go-json-experiment/json/internal/jsonwire"|' $X
    sed -i 's|"encoding/json/jsontext"|"github.com/go-json-experiment/json/jsontext"|' $X
    sed -i 's|"encoding/json"|"github.com/go-json-experiment/json/v1"|' $X
    sed -i 's|"internal/zstd"|"github.com/go-json-experiment/json/internal/zstd"|' $X
    goimports -w $X
done
sed -i 's/v2[.]struct/json.struct/' $JSONROOT/errors_test.go
sed -i 's|jsonv1 "github.com/go-json-experiment/json/v1"|jsonv1 "encoding/json"|' $JSONROOT/bench_test.go
git checkout internal/zstd # we still need local copy of zstd for testing
