package myjson

import (
	"github.com/go-json-experiment/json"
)

type optionsArshaler interface {
	OptionsArshal(dst *json.OptionsStruct)
}

type optionsCoder interface {
	OptionsCode(dst *json.OptionsStruct)
}
