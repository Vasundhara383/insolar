/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package transport

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/insolar/insolar/instrumentation/inslogger"
)

var (
	// ErrTimeout is returned when the operation timeout is exceeded.
	ErrTimeout = errors.New("timeout")
	// ErrChannelClosed is returned when the input channel is closed.
	ErrChannelClosed = errors.New("channel closed")
	// ErrChannelClosed is returned when the input channel is closed.
	ErrCancelled = errors.New("cancelled")
)

type Result interface{}

// CancelCallback is a callback function executed when cancelling Future.
type CancelCallback func(ctx context.Context)
type RunCallback func(context.Context) (Result, error)

// Future is network response future.
type Future interface {
	// Result is a channel to listen for future resultChan.
	Result() <-chan Result
	Error() <-chan error

	// WaitUntil gets the future resultChan from Result() channel with a timeout set to `duration`.
	WaitUntil(duration time.Duration) (Result, error)

	// Cancel closes all channels and cleans up underlying structures.
	Cancel(ctx context.Context)
}

type future struct {
	ctx context.Context

	resultChan chan Result
	errorChan  chan error

	runCallback    RunCallback
	cancelCallback CancelCallback

	canceled uint32
}

// NewFuture creates new Future.
func NewFuture(ctx context.Context, runCallback RunCallback, cancelCallback CancelCallback) Future {
	future := &future{
		ctx:        ctx,
		resultChan: make(chan Result, 1),
		errorChan:  make(chan error, 1),

		runCallback:    runCallback,
		cancelCallback: cancelCallback,
	}

	go future.run()
	return future
}

func (future *future) run() {
	result, err := future.runCallback(future.ctx)
	future.setResult(result, err)
	future.Cancel(future.ctx)
}

func (future *future) setResult(result Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			inslogger.FromContext(future.ctx).Debug("Caught panic on write future result. Possible future close in another context")
		}
	}()

	if err != nil {
		future.errorChan <- err
	} else {
		future.resultChan <- result
	}
}

func (future *future) checkResultError(result Result, err error, ok bool) (Result, error) {
	if !ok {
		if atomic.LoadUint32(&future.canceled) == 1 {
			return nil, ErrCancelled
		}

		return nil, ErrChannelClosed
	}
	future.Cancel(future.ctx)
	return result, nil
}

// Result returns resultChan packet channel.
func (future *future) Result() <-chan Result {
	return future.resultChan
}

func (future *future) Error() <-chan error {
	return future.errorChan
}

// WaitUntil gets the future resultChan from Result() channel with a timeout set to `duration`.
func (future *future) WaitUntil(duration time.Duration) (Result, error) {
	select {
	case result, ok := <-future.resultChan:
		return future.checkResultError(result, nil, ok)
	case err, ok := <-future.errorChan:
		return future.checkResultError(nil, err, ok)
	case <-time.After(duration):
		return nil, ErrTimeout
	}
}

// Cancel allows to cancel Future processing.
func (future *future) Cancel(ctx context.Context) {
	if atomic.CompareAndSwapUint32(&future.canceled, 0, 1) {
		if future.ctx != ctx {
			inslogger.FromContext(ctx).Debug("Future cancelled from other context")
		}

		close(future.resultChan)
		close(future.errorChan)
		future.cancelCallback(future.ctx)
	}
}
