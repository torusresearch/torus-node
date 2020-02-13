package common

import (
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-node/telemetry"
	"math/rand"
	"time"
)

func RandIndexes(min int, max int, length int) (res []int) {
	if max < min {
		return
	}

	rand.Seed(time.Now().UnixNano())
	p := rand.Perm(max - min)
	for _, r := range p[:length] {
		res = append(res, r+min)
	}
	return
}

func GetColumnPrimaryShare(matrix [][]PrimaryShare, columnIndex int) (column []PrimaryShare) {
	for _, row := range matrix {
		column = append(column, row[columnIndex])
	}
	return
}
func GetColumnPoint(matrix [][]common.Point, columnIndex int) (column []common.Point) {
	for _, row := range matrix {
		column = append(column, row[columnIndex])
	}
	return
}

type Semaphore struct {
	sem chan struct{}
}

type ReleasedState struct {
	Released bool
}

// ReleaseFunc - releases after acquire
type ReleaseFunc func()

func (s *Semaphore) Acquire() ReleaseFunc {
	telemetry.IncGauge("acquiredSemaphores")
	s.sem <- struct{}{}
	return func() {
		telemetry.DecGauge("acquiredSemaphores")
		<-s.sem
	}
}

func NewSemaphore(n int) *Semaphore {
	return &Semaphore{
		sem: make(chan struct{}, n),
	}
}
