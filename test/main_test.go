package test

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := loadLocalDefaults(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
