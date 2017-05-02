package client

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSum(t *testing.T) {
	assert.Equal(t, 1.0, Sum(-1).Merge(Sum(2)).Merge(Sum(0)).Merge(nil).Get())
}

func TestMin(t *testing.T) {
	assert.Equal(t, -1.0, Min(-1).Merge(Min(2)).Merge(Min(0)).Merge(nil).Get())
}

func TestMax(t *testing.T) {
	assert.Equal(t, 2.0, Max(-1).Merge(Max(2)).Merge(Max(0)).Merge(nil).Get())
}

func TestAvg(t *testing.T) {
	// Note - the subsequent types don't matter
	a1 := Avg(1).Merge(Sum(2))
	a2 := Avg(10).Merge(Sum(20))
	assert.Equal(t, 8.25, a1.Merge(a2).Merge(nil).Get())
}

func TestWeightedAvg(t *testing.T) {
	assert.Equal(t, 3.5, WeightedAvg(2, 5).Merge(WeightedAvg(4, 15)).Merge(nil).Get())
	assert.Equal(t, 2.0, WeightedAvg(2, 5).Get())
	assert.Equal(t, 0.0, WeightedAvg(2, 0).Merge(WeightedAvg(3, 0)).Merge(nil).Get())
}
