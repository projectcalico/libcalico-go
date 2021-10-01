// Copyright (c) 2021 Tigera, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logutils

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	fieldLogSkipped = "logs-skipped"
	fieldLogNextLog = "next-log"
)

// NewFirstAndIntervalLogger returns a FirstAndIntervalLogger which can be used for interval logging.
//
// Methods are essentially the same as the logrus logging methods, but there is no Panic or Fatal log since these don't
// make much sense for interval logging.
//
// The first log is always processed.  Subsequent logs are only processed if at least the interval time has passed since
// the last log request. Processed logs contain information about the number of skipped logs and the unix timestamp
// of that last log. Force() may be used to force the next log to be processed.  Note that sufficient log level is
// still required for a processed log to actually be written.
//
// Typical use might be as follows:
//
//   logger := NewFirstAndIntervalLogger(5 * time.Minute).WithField("key": "my-key")
//   for {
//     logger.Infof("Checking some stuff: %s", myStuff)
//     complete = doSomeStuff()
//     if complete {
//       break
//     }
//   }
//
//   // Use force to ensure our final log is printed and it contains the summary info about the number of skipped logs.
//   logger.Force().Info("Finished checking stuff")
//
func NewFirstAndIntervalLogger(interval time.Duration, logger *logrus.Logger) *FirstAndIntervalLogger {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &FirstAndIntervalLogger{
		nextLog: time.Now(),
		interval: interval,
		entry:    logrus.NewEntry(logger),
	}
}

type FirstAndIntervalLogger struct {
	nextLog  time.Time

	// Interval for logging.
	interval time.Duration

	// The logrus entry used for writing the log.
	entry    *logrus.Entry

	// The number skipped since the last processed log.
	skipped  int

	// Whether to force the next log to be processed.
	force    bool

	// Lock used to access to this data. This lock is never held while writing a log.
	lock     sync.RWMutex
}

func (logger *FirstAndIntervalLogger) logEntry() *logrus.Entry {
	now := time.Now()
	logger.lock.Lock()
	defer logger.lock.Unlock()
	if logger.force || now.Sub(logger.nextLog) >= 0 {
		nextLog := now.Add(logger.interval)
		entry := logger.entry.WithFields(logrus.Fields{
			fieldLogSkipped: logger.skipped,
			fieldLogNextLog: nextLog,
		})
		logger.force = false
		logger.nextLog = nextLog
		logger.skipped = 0
		return entry
	}
	logger.skipped++
	return nil
}

// Force forces the next log to be processed. Note that this does not force the log to be written since that is also
// dependent on the logging level.
func (logger *FirstAndIntervalLogger) Force() *FirstAndIntervalLogger {
	logger.lock.Lock()
	defer logger.lock.Unlock()

	// For this method, don't return a new logger, just update the existing one - the behavior will be less confusing.
	logger.force = true
	return logger
}

// WithError adds an error as single field (using the key defined in ErrorKey) to the FirstAndIntervalLogger.
func (logger *FirstAndIntervalLogger) WithError(err error) *FirstAndIntervalLogger {
	logger.lock.Lock()
	defer logger.lock.Unlock()
	return &FirstAndIntervalLogger{
		nextLog:  logger.nextLog,
		interval: logger.interval,
		skipped:  logger.skipped,
		entry:    logger.entry.WithError(err),
	}
}

// WithField adds a single field to the FirstAndIntervalLogger.
func (logger *FirstAndIntervalLogger) WithField(key string, value interface{}) *FirstAndIntervalLogger {
	logger.lock.Lock()
	defer logger.lock.Unlock()
	return &FirstAndIntervalLogger{
		nextLog:  logger.nextLog,
		interval: logger.interval,
		skipped:  logger.skipped,
		entry:    logger.entry.WithField(key, value),
	}
}

// WithFields adds a map of fields to the FirstAndIntervalLogger.
func (logger *FirstAndIntervalLogger) WithFields(fields logrus.Fields) *FirstAndIntervalLogger {
	logger.lock.Lock()
	defer logger.lock.Unlock()
	return &FirstAndIntervalLogger{
		nextLog:  logger.nextLog,
		interval: logger.interval,
		skipped:  logger.skipped,
		entry:    logger.entry.WithFields(fields),
	}
}

func (logger *FirstAndIntervalLogger) Debug(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Debug(args...)
	}
}

func (logger *FirstAndIntervalLogger) Print(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Print(args...)
	}
}

func (logger *FirstAndIntervalLogger) Info(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Info(args...)
	}
}

func (logger *FirstAndIntervalLogger) Warn(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Warn(args...)
	}
}

func (logger *FirstAndIntervalLogger) Warning(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Warning(args...)
	}
}

func (logger *FirstAndIntervalLogger) Error(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Error(args...)
	}
}

func (logger *FirstAndIntervalLogger) Debugf(format string, args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Debugf(format, args...)
	}
}

func (logger *FirstAndIntervalLogger) Infof(format string, args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Infof(format, args...)
	}
}

func (logger *FirstAndIntervalLogger) Printf(format string, args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Printf(format, args...)
	}
}

func (logger *FirstAndIntervalLogger) Warnf(format string, args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Warnf(format, args...)
	}
}

func (logger *FirstAndIntervalLogger) Warningf(format string, args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Warningf(format, args...)
	}
}

func (logger *FirstAndIntervalLogger) Errorf(format string, args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Errorf(format, args...)
	}
}

// Entry Println family functions

func (logger *FirstAndIntervalLogger) Debugln(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Debugln(args...)
	}
}

func (logger *FirstAndIntervalLogger) Infoln(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Infoln(args...)
	}
}

func (logger *FirstAndIntervalLogger) Println(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Println(args...)
	}
}

func (logger *FirstAndIntervalLogger) Warnln(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Warnln(args...)
	}
}

func (logger *FirstAndIntervalLogger) Warningln(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Warningln(args...)
	}
}

func (logger *FirstAndIntervalLogger) Errorln(args ...interface{}) {
	if entry := logger.logEntry(); entry != nil {
		entry.Errorln(args...)
	}
}
