package pt700

import (
	"fmt"
	"image"
	"strings"
)

type Status struct {
	Err1       Error1
	Err2       Error2
	MediaWidth MediaWidth
	MediaType  MediaType
	Type       StatusType
	Phase      PhaseType
}

// Err returns an error representing this status, or nil if there is no error.
func (s Status) Err() error {
	if s.Err1 != 0 {
		return fmt.Errorf("%v", s.Err1)
	}
	if s.Err2 != 0 {
		return fmt.Errorf("%v", s.Err2)
	}
	return nil
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
	return bitfieldString(e, []bitfield[Error1]{
		{Err1NoMedia, "NoMedia"},
		{Err1CutterJam, "CutterJam"},
		{Err1WeakBatteries, "WeakBatteries"},
		{Err1HighVoltageAdapter, "HighVoltageAdapter"},
	})
}

type bitfield[B ~byte] struct {
	b    B
	name string
}

func bitfieldString[B ~byte](val B, fields []bitfield[B]) string {
	if val == 0 {
		return "None"
	}

	var (
		set  []string
		seen B
	)

	for _, field := range fields {
		if val&(field.b) != 0 {
			set = append(set, field.name)
			seen = seen | field.b
		}
	}

	if unknown := val & ^seen; unknown != 0 {
		set = append(set, fmt.Sprintf("Unknown(0x%x)", unknown))
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

func (e Error2) String() string {
	return bitfieldString(e, []bitfield[Error2]{
		{Err2ReplaceMedia, "ReplaceMedia"},
		{Err2CoverOpen, "CoverOpen"},
		{Err2Overheating, "Overheating"},
	})
}

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

func (w MediaWidth) pixels() (int, error) {
	switch w {
	case Width3_5:
		return 24, nil
	case Width6:
		return 32, nil
	case Width9:
		return 50, nil
	case Width12:
		return 70, nil
	case Width18:
		return 112, nil
	case Width24:
		return 128, nil
	default:
		return 0, fmt.Errorf("unknown tape width")
	}
}

// The smallest image that can be printed (assuming no margins).
// Longer (.Dx()) images can be printed, but not wider (.Dy()) images.
func (w MediaWidth) MinBounds() (image.Rectangle, error) {
	width, err := w.pixels()
	if err != nil {
		return image.Rectangle{}, nil
	}

	return image.Rectangle{
		image.Point{0, 0},
		// Brother PDF 2.3.3 (I think, later it says 24.5mm..)
		image.Point{172, width},
	}, nil
}

// PTouch printers expect full width data even with narrow media.
// Software has to explicitly skip pins outside of the print area.
func (w MediaWidth) unusedPins(printerPins int) (int, error) {
	width, err := w.pixels()
	if err != nil {
		return 0, err
	}

	return (128 - width) / 2, nil
}

func (w MediaWidth) DPI() int {
	// Brother PDF 2.3.4
	return 180
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

type StatusType uint8

const (
	StatusReplyToRequest StatusType = iota
	StatusPrintingCompleted
	StatusErrorOccurred
	StatusExitIFMode
	StatusTurnedOff
	StatusNotification
	StatusPhaseChange
)

func (t StatusType) String() string {
	switch t {
	case StatusReplyToRequest:
		return "ReplyToStatusRequest"
	case StatusPrintingCompleted:
		return "PrintingCompleted"
	case StatusErrorOccurred:
		return "StatusErrorOccurred"
	case StatusExitIFMode:
		return "StatusExitIFMode"
	case StatusTurnedOff:
		return "StatusTurnedOff"
	case StatusNotification:
		return "StatusNotification"
	case StatusPhaseChange:
		return "StatusPhaseChange"
	default:
		return fmt.Sprintf("Unknown(0x%x)", uint8(t))
	}
}

type PhaseType uint8

const (
	PhaseEditing PhaseType = iota
	PhasePrinting
)

func (p PhaseType) String() string {
	switch p {
	case PhaseEditing:
		return "Editing"
	case PhasePrinting:
		return "Printing"
	default:
		return fmt.Sprintf("PhaseType(0x%x)", uint8(p))
	}
}
