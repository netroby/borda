package client

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFloat(t *testing.T) {
	assert.Equal(t, 1.0, Float(-1).Plus(Float(2)).Plus(Float(0)).Plus(nil).Get())
}

func TestMin(t *testing.T) {
	assert.Equal(t, -1.0, Min(-1).Plus(Min(2)).Plus(Min(0)).Plus(nil).Get())
}

func TestMax(t *testing.T) {
	assert.Equal(t, 2.0, Max(-1).Plus(Max(2)).Plus(Max(0)).Plus(nil).Get())
}

func TestAvg(t *testing.T) {
	// Note - the subsequent types don't matter
	a1 := Avg(1).Plus(Float(2))
	a2 := Avg(10).Plus(Float(20))
	assert.Equal(t, 8.25, a1.Plus(a2).Plus(nil).Get())
}

func TestWeightedAvg(t *testing.T) {
	assert.Equal(t, 3.5, WeightedAvg(2, 5).Plus(WeightedAvg(4, 15)).Plus(nil).Get())
	assert.Equal(t, 2.0, WeightedAvg(2, 5).Get())
	assert.Equal(t, 0.0, WeightedAvg(2, 0).Plus(WeightedAvg(3, 0)).Plus(nil).Get())
}
