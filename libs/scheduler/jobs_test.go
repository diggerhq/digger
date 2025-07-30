package scheduler

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUniqueLayersForJobs(t *testing.T) {
	jobs := []Job{
		{
			Layer: 0,
		},
		{
			Layer: 1,
		},
		{
			Layer: 1,
		},
	}

	cnt, uniqueLayers := CountUniqueLayers(jobs)
	assert.Equal(t, cnt, uint(2))
	assert.Equal(t, uniqueLayers, []uint{0, 1})
}
