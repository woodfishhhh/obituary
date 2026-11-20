//go:build linux && desktop

package app

import (
	"log"
	"time"
)

const quietPeriod = 10 * time.Minute

type errorLog struct {
	logger *log.Logger
	last   map[string]string
	at     map[string]time.Time
}

func (e *errorLog) report(key string, err error) {
	msg := err.Error()
	if e.last[key] == msg && time.Since(e.at[key]) < quietPeriod {
		return
	}
	e.last[key], e.at[key] = msg, time.Now()
	e.logger.Printf("%s: %v", key, err)
}

func (e *errorLog) clear(key string) { delete(e.last, key) }
