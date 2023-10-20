package jsonopts

import (
	"github.com/go-json-experiment/json/internal"
)

type OptionsArshaler interface {
	OptionsArshal(dst *Struct, _ internal.NotForPublicUse)
}

type OptionsCoder interface {
	OptionsCode(dst *Struct, _ internal.NotForPublicUse)
}
