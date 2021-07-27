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

package backoff

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

type BackoffHandler interface {
	// Returns the current backoff time.
	Backoff() time.Duration

	// Runs the main logic which should watch the update channel
	// and increment the backoff time accordingly.
	Run(context.Context)
}

// backoffHandler implements a single manager to handle backoff times for
// multiple clients. It implements a leaky bucket algorithm where the leak
// rate and fill rate are configurable. If the bucket overflows (hits max),
// it stops increasing the backoff.
type backoffHandler struct {
	ticker     *time.Ticker
	backoff    time.Duration
	minBackoff time.Duration
	maxBackoff time.Duration
	leak       time.Duration
	backoffFn  func(time.Duration) time.Duration
	updates    chan time.Time
	lastUpdate time.Time
	lock       bool
}

// Creates a backoff handler which will increase the backoff time by increment each time it is called.
func NewLinearBackoffHandler(updates chan time.Time, leakTime, leakPeriod, minBackoff, maxBackoff, increment time.Duration) BackoffHandler {
	backoffFn := func(t time.Duration) time.Duration {
		return t + increment
	}
	return NewBackoffHandler(updates, leakTime, leakPeriod, minBackoff, maxBackoff, backoffFn)
}

func NewBackoffHandler(updates chan time.Time, leakTime, leakPeriod, minBackoff, maxBackoff time.Duration, backoffFn func(time.Duration) time.Duration) BackoffHandler {
	return &backoffHandler{
		ticker:     time.NewTicker(leakPeriod),
		minBackoff: minBackoff,
		maxBackoff: maxBackoff,
		leak:       leakTime,
		backoffFn:  backoffFn,
		updates:    updates,
		backoff:    minBackoff,
	}
}

// Backoff returns the current backoff time.
func (bh *backoffHandler) Backoff() time.Duration {
	return bh.backoff
}

// Run is the main logic loop for the backoff handler. It will update the backoff only if the timestamp
// sent by the caller is newer than the last time it updated the backoff. This should prevent all clients
// from triggering backoff updates at the same time.
func (bh *backoffHandler) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("Context is done. Returning")
			return
		case <-bh.ticker.C:
			update := bh.backoff - bh.leak
			// Unlock the handler after waiting 1 ticker period
			bh.lock = false
			if update < bh.minBackoff {
				bh.backoff = bh.minBackoff
			} else {
				bh.backoff = update
			}
		case timestamp := <-bh.updates:
			// Only update the backoff if the timestamp is after the last update time.
			if !bh.lock && bh.lastUpdate.Before(timestamp) {
				bh.lock = true
				update := bh.backoffFn(bh.backoff)
				if update < bh.maxBackoff {
					log.Debug("Backoff limit has reached its max. Do not update the backoff time")
					bh.backoff = bh.maxBackoff
				} else {
					update := bh.backoffFn(bh.backoff)
					log.Debug("Updating backoff time from %s to %s", bh.backoff, update)
					bh.backoff = update
					bh.lastUpdate = time.Now()
				}
			}
		}
	}
}
