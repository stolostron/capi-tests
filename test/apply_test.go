package test

import (
	"errors"
	"reflect"
	"testing"
)

func TestApplyFilesInOrderStopsAfterFailure(t *testing.T) {
	files := []string{"credentials.yaml", "cluster.yaml", "machinepool.yaml"}
	injectedErr := errors.New("injected apply failure")
	var attempted []string

	applied, err := applyFilesInOrder(files, func(file string) error {
		attempted = append(attempted, file)
		if file == files[1] {
			return injectedErr
		}
		return nil
	})

	if !errors.Is(err, injectedErr) {
		t.Fatalf("applyFilesInOrder() error = %v, want injected failure", err)
	}

	if want := []string{files[0]}; !reflect.DeepEqual(applied, want) {
		t.Fatalf("applied files = %v, want %v", applied, want)
	}

	if want := []string{files[0], files[1]}; !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted files = %v, want %v", attempted, want)
	}
}

func TestApplyFilesInOrderReportsNoAppliedFilesWhenFirstApplyFails(t *testing.T) {
	files := []string{"credentials.yaml", "cluster.yaml"}
	injectedErr := errors.New("injected first apply failure")
	var attempted []string

	applied, err := applyFilesInOrder(files, func(file string) error {
		attempted = append(attempted, file)
		return injectedErr
	})

	if !errors.Is(err, injectedErr) {
		t.Fatalf("applyFilesInOrder() error = %v, want injected failure", err)
	}

	if len(applied) != 0 {
		t.Fatalf("applied files = %v, want none", applied)
	}

	if want := []string{files[0]}; !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted files = %v, want %v", attempted, want)
	}
}
