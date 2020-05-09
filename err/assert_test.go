package err

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssertPanics(t *testing.T) {
	assert.Panics(t, func() { Assert(false) })
}

func TestAssertDoesntPanic(t *testing.T) {
	assert.NotPanics(t, func() { Assert(true) })
}
