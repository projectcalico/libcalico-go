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

package backoff_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectcalico/libcalico-go/lib/backend/backoff"
)

var _ = Describe("Test the backend backoff handler", func() {
	Context("With a linear backoff handler that backsoff by 100 ms every call and decreases by 50 ms per second", func() {
		var updateCallCount int

		It("Should increment the backoff correctly", func() {
			updates := make(chan time.Time)
			defer close(updates)
			fn := func(t time.Duration) time.Duration {
				updateCallCount++
				return t + 100*time.Millisecond
			}
			bh := backoff.NewBackoffHandler(updates, 50*time.Millisecond, 1*time.Second, 0*time.Millisecond, 200*time.Millisecond, fn)
			ctx := context.Background()
			go bh.Run(ctx)

			Expect(bh.Backoff()).To(Equal(0 * time.Millisecond))
			updates <- time.Now()
			Expect(bh.Backoff()).To(Equal(100 * time.Millisecond))
			time.Sleep(2 * time.Second)
		})

		It("Backoff should decrease over time", func() {
			updates := make(chan time.Time)
			defer close(updates)
			fn := func(t time.Duration) time.Duration {
				updateCallCount++
				return t + 100*time.Millisecond
			}
			bh := backoff.NewBackoffHandler(updates, 50*time.Millisecond, 1*time.Second, 0*time.Millisecond, 200*time.Millisecond, fn)
			ctx := context.Background()
			go bh.Run(ctx)

			updates <- time.Now()
			time.Sleep(1 * time.Second)
			Expect(bh.Backoff()).To(Equal(50 * time.Millisecond))
			time.Sleep(1 * time.Second)
		})

		It("Should only backoff once per leak period", func() {
			updates := make(chan time.Time)
			defer close(updates)
			fn := func(t time.Duration) time.Duration {
				updateCallCount++
				return t + 100*time.Millisecond
			}
			bh := backoff.NewBackoffHandler(updates, 50*time.Millisecond, 1*time.Second, 0*time.Millisecond, 200*time.Millisecond, fn)
			ctx := context.Background()
			go bh.Run(ctx)

			updateCallCount = 0
			updates <- time.Now()
			updates <- time.Now()
			updates <- time.Now()
			Expect(bh.Backoff()).To(Equal(100 * time.Millisecond))
			Expect(updateCallCount).To(Equal(1))
		})

		It("Should never have a backoff greater than the maximum", func() {
			for i := 0; i < 6; i++ {
				updates <- time.Now()
				time.Sleep(1 * time.Second)
			}
			Expect(bh.Backoff()).To(Equal(200 * time.Millisecond))
		})
	})
})
