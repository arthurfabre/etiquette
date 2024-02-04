package pt700

import (
	"fmt"
	"strings"
)

type Status struct {
	Err1  Error1
	Err2  Error2
	Width MediaWidth
	Type  MediaType
}

type Error1 byte

const (
	Err1NoMedia Error1 = 1 << iota
	_
	Err1CutterJam
	Err1WeakBatteries
	_
	_
	Err1HighVoltageAdapter
	_
)

func (e Error1) String() string {
	if e == 0 {
		return "None"
	}

	var set []string

	for i := 0; i < 8; i++ {
		if e&(1<<i) == 0 {
			continue
		}
		err := Error1(1 << i)

		switch err {
		case Err1NoMedia:
			set = append(set, "NoMedia")
		case Err1CutterJam:
			set = append(set, "CutterJam")
		case Err1WeakBatteries:
			set = append(set, "WeakBatteries")
		case Err1HighVoltageAdapter:
			set = append(set, "HighVoltageAdapter")
		default:
			set = append(set, fmt.Sprintf("Unknown(%x)", byte(err)))
		}
	}

	return strings.Join(set, "|")
}

type Error2 byte

const (
	Err2ReplaceMedia Error2 = 1 << iota
	_
	_
	_
	Err2CoverOpen
	Err2Overheating
	_
	_
)

type MediaWidth uint8

const (
	WidthNoMedia MediaWidth = 0
	Width3_5                = 4
	Width6                  = 6
	Width9                  = 9
	Width12                 = 12
	Width18                 = 18
	Width24                 = 24
)

func (w MediaWidth) String() string {
	switch w {
	case WidthNoMedia:
		return "NoMedia"
	case Width3_5:
		return "3.5mm"
	case Width6, Width9, Width12, Width18, Width24:
		return fmt.Sprintf("%dmm", w)
	default:
		return fmt.Sprintf("Unknown(%d)", uint8(w))
	}
}

type MediaType byte

const (
	TypeNoMedia      MediaType = 0x00
	TypeLaminated              = 0x01
	TypeNonLaminated           = 0x03
	TypeHeatShrink21           = 0x11
	TypeHeatShrink31           = 0x17
	TypeIncompatible           = 0xFF
)

func (t MediaType) String() string {
	switch t {
	case TypeNoMedia:
		return "None"
	case TypeLaminated:
		return "Laminated"
	case TypeNonLaminated:
		return "NonLaminated"
	case TypeHeatShrink21:
		return "HeatShrink2:1"
	case TypeHeatShrink31:
		return "HeatShrink3:1"
	case TypeIncompatible:
		return "Incompatible"
	default:
		return fmt.Sprintf("Unknown(0x%x)", byte(t))
	}
}
