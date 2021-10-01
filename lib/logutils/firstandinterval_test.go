// Copyright (c) 2016-2018 Tigera, Inc. All rights reserved.
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

package logutils_test

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"os"
	"time"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/projectcalico/libcalico-go/lib/logutils"
)

// A mock log formatter that simply serves to count log invocations.
type mockLogFormatter struct {
	count int
	entry *log.Entry
}

func (s *mockLogFormatter) Format(e *log.Entry) ([]byte, error) {
	s.count++
	s.entry = e
	return nil, nil
}

var _ = DescribeTable("First and interval logging",
	func(level log.Level, logfn func(logger *FirstAndIntervalLogger)) {
		counter := &mockLogFormatter{}
		logrusLogger := &log.Logger{
			Out:       os.Stderr,
			Formatter: counter,
			Hooks:     make(log.LevelHooks),
			Level:     log.DebugLevel,
		}
		logger := NewFirstAndIntervalLogger(200 * time.Millisecond, logrusLogger)
		logger = logger.WithError(errors.New("error"))
		logger = logger.WithField("a", 1)
		logger = logger.WithFields(log.Fields{"b": 2, "c": "3"})

		// First log will be written.
		logfn(logger)
		Expect(counter.count).To(Equal(1))
		Expect(counter.entry.Data).To(HaveKeyWithValue("a", 1))
		Expect(counter.entry.Data).To(HaveKeyWithValue("b", 2))
		Expect(counter.entry.Data).To(HaveKeyWithValue("c", "3"))
		Expect(counter.entry.Data).To(HaveKeyWithValue("logs-skipped", 0))
		Expect(counter.entry.Data).To(HaveKey("next-log"))
		Expect(counter.entry.Data).To(HaveKey("error"))

		// Next two log will be skipped.
		logfn(logger)
		logfn(logger)
		Expect(counter.count).To(Equal(1))

		// Wait for logging interval.
		time.Sleep(200 * time.Millisecond)

		// Next log will be written.
		logfn(logger)
		Expect(counter.count).To(Equal(2))
		Expect(counter.entry.Data).To(HaveKeyWithValue("a", 1))
		Expect(counter.entry.Data).To(HaveKeyWithValue("b", 2))
		Expect(counter.entry.Data).To(HaveKeyWithValue("c", "3"))
		Expect(counter.entry.Data).To(HaveKeyWithValue("logs-skipped", 2))
		Expect(counter.entry.Data).To(HaveKey("next-log"))
		Expect(counter.entry.Data).To(HaveKey("error"))

		// Force, so next log will also be written.
		logger.Force()
		logfn(logger)
		Expect(counter.count).To(Equal(3))
		Expect(counter.entry.Level).To(Equal(level))
		Expect(counter.entry.Data).To(HaveKeyWithValue("a", 1))
		Expect(counter.entry.Data).To(HaveKeyWithValue("b", 2))
		Expect(counter.entry.Data).To(HaveKeyWithValue("c", "3"))
		Expect(counter.entry.Data).To(HaveKeyWithValue("logs-skipped", 0))
		Expect(counter.entry.Data).To(HaveKey("next-log"))
		Expect(counter.entry.Data).To(HaveKey("error"))
	},
	Entry("Debug", log.DebugLevel, func(l *FirstAndIntervalLogger) {l.Debug("log", "now")}),
	Entry("Print", log.InfoLevel, func(l *FirstAndIntervalLogger) {l.Print("log", "now")}),
	Entry("Info", log.InfoLevel, func(l *FirstAndIntervalLogger) {l.Info("log", "now")}),
	Entry("Warn", log.WarnLevel, func(l *FirstAndIntervalLogger) {l.Warn("log", "now")}),
	Entry("Warning", log.WarnLevel, func(l *FirstAndIntervalLogger) {l.Warning("log", "now")}),
	Entry("Error", log.ErrorLevel, func(l *FirstAndIntervalLogger) {l.Error("log", "now")}),
	Entry("Debugf", log.DebugLevel, func(l *FirstAndIntervalLogger) {l.Debugf("log %s", "hello")}),
	Entry("Printf", log.InfoLevel, func(l *FirstAndIntervalLogger) {l.Printf("log %s", "hello")}),
	Entry("Infof", log.InfoLevel, func(l *FirstAndIntervalLogger) {l.Infof("log %s", "hello")}),
	Entry("Warnf", log.WarnLevel, func(l *FirstAndIntervalLogger) {l.Warnf("log %s", "hello")}),
	Entry("Warningf", log.WarnLevel, func(l *FirstAndIntervalLogger) {l.Warningf("log %s", "hello")}),
	Entry("Errorf", log.ErrorLevel, func(l *FirstAndIntervalLogger) {l.Errorf("log %s", "hello")}),
	Entry("Debugln", log.DebugLevel, func(l *FirstAndIntervalLogger) {l.Debugln("log", "now")}),
	Entry("Println", log.InfoLevel, func(l *FirstAndIntervalLogger) {l.Println("log", "now")}),
	Entry("Infoln", log.InfoLevel, func(l *FirstAndIntervalLogger) {l.Infoln("log", "now")}),
	Entry("Warnln", log.WarnLevel, func(l *FirstAndIntervalLogger) {l.Warnln("log", "now")}),
	Entry("Warningln", log.WarnLevel, func(l *FirstAndIntervalLogger) {l.Warningln("log", "now")}),
	Entry("Errorln", log.ErrorLevel, func(l *FirstAndIntervalLogger) {l.Errorln("log", "now")}),
)
