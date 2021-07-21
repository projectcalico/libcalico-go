// Copyright (c) 2021 Tigera, Inc. All rights reserved.

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

// Package grstacktrace contains utilities for getting go routine stack traces from running go programs. A go program
// simple needs to run grstacktrace.MaybeRunServer() and set the GR_STACK_TRACE_SERVER_ENABLED environment variable
// to true and a server will be run in the background that can be accessed to get a go routine stack trace on the
// running program.
//
// The default host and port this server runs on is localhost:8989, but is configurable using the
// GR_STACK_TRACE_SERVER_HOST and GR_STACK_TRACE_SERVER_PORT environment variables.
package grstacktrace

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

type config struct {
	GRStackTraceServerEnabled bool   `default:"false" split_words:"true"`
	GRStackTraceServerHost    string `default:"localhost" split_words:"true"`
	GRStackTraceServerPort    string `default:"8989" split_words:"true"`
}

type server struct{}

// ServeHTTP returns the responds to any request with the go routine stack trace created by runtime.Stack.
func (s server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)

	if _, err := fmt.Fprintf(writer, string(buf)); err != nil {
		log.WithError(err).Error("failed to print out go routine stack trace")
	}
}

// MaybeRunServer checks if the GR_STACK_TRACE_SERVER_ENABLED env variable has been set, and if it has been, instantiates
// and runs and instance of `server` on the host and port specified by the env variables GR_STACK_TRACE_SERVER_HOST and
// GR_STACK_TRACE_SERVER_PORT
func MaybeRunServer() {
	cfg := &config{}
	if err := envconfig.Process("", cfg); err != nil {
		log.WithError(err).Error("failed to process stacktrace handler configuration")
		return
	}

	if cfg.GRStackTraceServerEnabled {
		address := fmt.Sprintf("%s:%s", cfg.GRStackTraceServerHost, cfg.GRStackTraceServerPort)
		log.Infof("Go routine stacktrace handler running on %s", address)

		go func() {
			if err := http.ListenAndServe(address, &server{}); err != nil {
				log.WithError(err).Error("failed to serve go routine stacktrace handler")
			}
		}()
	} else {
		log.Info("Stacktrace handler not enabled.")
	}
}
