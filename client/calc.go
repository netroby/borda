package client

import (
	"encoding/json"
)

// Val represents a value that can be reduced by a reducing submitter.
type Val interface {
	// Get gets the final float64 value
	Get() float64

	// Plus adds another value to this one, using logic specific to the type of
	// value.
	Plus(b Val) Val
}

// Float is float value that gets reduced by plain addition.
type Float float64

func (a Float) Plus(b Val) Val {
	if b == nil {
		return a
	}
	i := a.Get()
	ii := b.Get()
	return Float(i + ii)
}

func (a Float) Get() float64 {
	return float64(a)
}

// Min is a float value that gets reduced by taking the lowest value.
type Min float64

func (a Min) Plus(b Val) Val {
	if b == nil {
		return a
	}
	i := a.Get()
	ii := b.Get()
	if i < ii {
		return a
	}
	return b
}

func (a Min) Get() float64 {
	return float64(a)
}

// Max is a float value that gets reduced by taking the highest value.
type Max float64

func (a Max) Plus(b Val) Val {
	if b == nil {
		return a
	}
	i := a.Get()
	ii := b.Get()
	if i > ii {
		return a
	}
	return b
}

func (a Max) Get() float64 {
	return float64(a)
}

// Avg creates a value that gets reduced by taking the arithmetic mean of the
// values.
func Avg(val float64) Val {
	return avg{val, 1}
}

// WeightedAvg is like Avg but the value is weighted by a given weight.
func WeightedAvg(val float64, weight float64) Val {
	return avg{val * weight, weight}
}

// avg holds the total plus a count in order to calculate the arithmatic mean
type avg [2]float64

func (a avg) Plus(_b Val) Val {
	if _b == nil {
		return a
	}
	switch b := _b.(type) {
	case avg:
		return avg{a[0] + b[0], a[1] + b[1]}
	default:
		return avg{a[0] + b.Get(), a[1] + 1}
	}
}

func (a avg) Get() float64 {
	if a[1] == 0 {
		return 0
	}
	return a[0] / a[1]
}

// avg is marshalled to JSON as its final float64 value.
func (a avg) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Get())
}
