package main

import (
	"testing"
)
import "github.com/stretchr/testify/assert"

func TestDiggerConfigFileDoesNotExist(t *testing.T) {
	dg, err := NewDiggerConfig("")
	assert.Equal(t, err, nil, "expected error to be not nil")
	assert.Equal(t, dg.Projects[0].Name, "default", "expected default project to have name 'default'")
	assert.Equal(t, dg.Projects[0].Dir, ".", "expected default project dir to be '.'")
}
