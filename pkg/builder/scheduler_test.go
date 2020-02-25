package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNextJobID(t *testing.T) {
	s := newScheduler()

	assert.Equal(t, int64(1), s.getNextJobID())
	assert.Equal(t, int64(2), s.getNextJobID())
	assert.Equal(t, int64(3), s.getNextJobID())

	assert.Equal(t, int64(4), s.nextJobID)
}

func TestScheduleJob(t *testing.T) {
	s := newScheduler()

	assert.Nil(t, s.scheduleJob("test", 1))
	assert.Nil(t, s.scheduleJob("test", 5))
	assert.Equal(t, ErrJobOutdated, s.scheduleJob("test", 2))
	assert.Nil(t, s.scheduleJob("test", 6))
	assert.Nil(t, s.scheduleJob("test", 6))
}
