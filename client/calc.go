package client

import (
	"encoding/json"
)

// Val represents a value that can be reduced by a reducing submitter.
type Val interface {
	// Get gets the final float64 value
	Get() float64

	// Merges merges another value with this one, using logic specific to the type
	// of value.
	Merge(b Val) Val
}

// Sum is float value that gets reduced by plain addition.
type Sum float64

func (a Sum) Merge(b Val) Val {
	if b == nil {
		return a
	}
	i := a.Get()
	ii := b.Get()
	return Sum(i + ii)
}

func (a Sum) Get() float64 {
	return float64(a)
}

// Float is an alias for Sum, it is deprecated, using Sum instead
type Float Sum

func (a Float) Merge(b Val) Val {
	if b == nil {
		return a
	}
	i := a.Get()
	ii := b.Get()
	return Sum(i + ii)
}

func (a Float) Get() float64 {
	return float64(a)
}

// Min is a float value that gets reduced by taking the lowest value.
type Min float64

func (a Min) Merge(b Val) Val {
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

func (a Max) Merge(b Val) Val {
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

func (a avg) Merge(_b Val) Val {
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
