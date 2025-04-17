//go:build !windows

package main

import (
	"os"
	"path/filepath"

	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr/eventstore/mmm"
	"github.com/rs/zerolog"
)

func doMmmInit(path string) (eventstore.Store, error) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	}))
	mmmm := mmm.MultiMmapManager{
		Dir:    filepath.Dir(path),
		Logger: &logger,
	}
	if err := mmmm.Init(); err != nil {
		return nil, err
	}
	il := &mmm.IndexingLayer{}
	if err := mmmm.EnsureLayer(filepath.Base(path), il); err != nil {
		return nil, err
	}
	return il, nil
}
