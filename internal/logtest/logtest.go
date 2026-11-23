// Package logtest provides a recording slog logger for tests.
//
// The recorder is a slog.JSONHandler that writes into a buffer, so a test sees
// each record as the standard library encodes it: a Duration attribute is a
// number of nanoseconds and an error attribute is its message.
package logtest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Record is one log record read back from the recorder.
type Record struct {
	Level   slog.Level
	Message string
	// Attrs holds the record's attributes, decoded from JSON.
	Attrs map[string]any
	// Text is the whole encoded record, for a test that checks that a value
	// does not appear anywhere in it.
	Text string
}

// Has reports whether the record has an attribute named key.
func (r Record) Has(key string) bool {
	_, ok := r.Attrs[key]
	return ok
}

// Str returns the attribute named key if it is a string, and "" otherwise.
func (r Record) Str(key string) string {
	s, _ := r.Attrs[key].(string)
	return s
}

// Duration returns the attribute named key, written with slog.Duration, and
// whether the record has it.
func (r Record) Duration(key string) (time.Duration, bool) {
	ns, ok := r.Attrs[key].(float64)
	return time.Duration(ns), ok
}

// Recorder collects the records a logger built from it writes. It is safe for
// concurrent use.
type Recorder struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// New returns an empty Recorder.
func New() *Recorder { return &Recorder{} }

// Logger returns a logger that writes every level to r.
func (r *Recorder) Logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(r, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
}

// Write stores one encoded record. The JSON handler calls it once per record.
func (r *Recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

// Records returns the records written so far, in write order. It panics when
// a line is not the JSON a slog.JSONHandler writes.
func (r *Recorder) Records() []Record {
	r.mu.Lock()
	lines := strings.Split(strings.TrimSpace(r.buf.String()), "\n")
	r.mu.Unlock()
	var recs []Record
	for _, line := range lines {
		if line == "" {
			continue
		}
		attrs := map[string]any{}
		if err := json.Unmarshal([]byte(line), &attrs); err != nil {
			panic("logtest: " + err.Error())
		}
		levelText, _ := attrs[slog.LevelKey].(string)
		var level slog.Level
		if err := level.UnmarshalText([]byte(levelText)); err != nil {
			panic("logtest: " + err.Error())
		}
		msg, _ := attrs[slog.MessageKey].(string)
		delete(attrs, slog.LevelKey)
		delete(attrs, slog.MessageKey)
		recs = append(recs, Record{Level: level, Message: msg, Attrs: attrs, Text: line})
	}
	return recs
}
