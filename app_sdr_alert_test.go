package main

import (
	"errors"
	"testing"
)

func TestAppSDRAlertState(t *testing.T) {
	app := &App{}

	initial := app.GetSDRAlert()
	if initial.Active || initial.FailCount != 0 {
		t.Fatalf("expected initial alert inactive with count 0, got %+v", initial)
	}

	app.recordSDROpenFailure(errors.New("usb device busy"))
	first := app.GetSDRAlert()
	if !first.Active || first.FailCount != 1 || first.Message != "usb device busy" {
		t.Fatalf("unexpected alert after 1st failure: %+v", first)
	}

	app.recordSDROpenFailure(errors.New("no device found"))
	second := app.GetSDRAlert()
	if !second.Active || second.FailCount != 2 || second.Message != "no device found" {
		t.Fatalf("unexpected alert after 2nd failure: %+v", second)
	}

	app.clearSDROpenFailure()
	cleared := app.GetSDRAlert()
	if cleared.Active || cleared.FailCount != 0 || cleared.Message != "" {
		t.Fatalf("expected cleared alert, got %+v", cleared)
	}
}
