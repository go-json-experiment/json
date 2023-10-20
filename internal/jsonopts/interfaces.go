package jsonopts

import (
	"github.com/go-json-experiment/json/internal"
)

type optionsArshaler interface {
	OptionsArshal(dst *Struct, _ internal.NotForPublicUse)
}

type optionsCoder interface {
	OptionsCode(dst *Struct, _ internal.NotForPublicUse)
}
