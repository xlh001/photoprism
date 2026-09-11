package api

import (
	"github.com/photoprism/photoprism/internal/event"
	"github.com/photoprism/photoprism/pkg/clean"
)

var log = event.Log

// logErr logs an error if err is not nil.
func logErr(prefix string, err error) {
	if err != nil {
		log.Errorf("%s: %s", prefix, clean.Error(err))
	}
}

// logWarn logs a warning if err is not nil.
func logWarn(prefix string, err error) {
	if err != nil {
		log.Warnf("%s: %s", prefix, clean.Error(err))
	}
}
