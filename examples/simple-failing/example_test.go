//go:build e2e

package example

import (
	"testing"
)

func TestExample1(t *testing.T) {
	t.Log("Hello, world 1!")
}

func TestExample2(t *testing.T) {
	t.Log("Hello, world 2!")
	t.Fail()
}
