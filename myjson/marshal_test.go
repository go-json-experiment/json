package myjson_test

import (
	"reflect"
	"testing"

	"github.com/go-json-experiment/json"
	"myjson"
)

type Foo struct {
	Name    string
	Age     int
	Citizen bool
	Map     map[string]int
}

func TestMarshalViaStruct(t *testing.T) {
	tests := []struct {
		name    string
		in      any
		wantOut []byte
		wantErr bool
		opts    []json.Options
	}{
		{
			name:    "No options",
			in:      Foo{Name: "John", Age: 41, Citizen: true},
			wantOut: []byte(`{"Name":"John","Age":41,"Citizen":true,"Map":{}}`),
			wantErr: false,
			opts:    nil,
		},
		{
			name:    "Format Nil Map as Null — Variadic",
			in:      Foo{Name: "John", Age: 41, Citizen: true},
			wantOut: []byte(`{"Name":"John","Age":41,"Citizen":true,"Map":null}`),
			wantErr: false,
			opts:    []json.Options{json.FormatNilMapAsNull(true)},
		},
		{
			name:    "Format Nil Map as Null — Struct",
			in:      Foo{Name: "John", Age: 41, Citizen: true},
			wantOut: []byte(`{"Name":"John","Age":41,"Citizen":true,"Map":null}`),
			wantErr: false,
			opts: []json.Options{&myjson.MarshalOptions{
				FormatNilMapAsNull: true,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, err := json.Marshal(tt.in, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotOut, tt.wantOut) {
				t.Errorf("Marshal() gotOut = %s, want %v", string(gotOut), string(tt.wantOut))
			}
		})
	}
}
