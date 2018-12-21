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
	"sync"
)

type futureManagerImpl struct {
	mutex   sync.RWMutex
	futures map[Sequence]Future

	sequenceGenerator sequenceGenerator
}

func newFutureManagerImpl() *futureManagerImpl {
	return &futureManagerImpl{
		futures:           make(map[Sequence]Future),
		sequenceGenerator: newSequenceGenerator(),
	}
}

func (fm *futureManagerImpl) Create(ctx context.Context, callback RunCallback) Future {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	id := fm.sequenceGenerator.Generate()
	futureCtx := context.WithValue(ctx, FutureID, Sequence(id))

	future := NewFuture(futureCtx, callback, func(ctx context.Context) {
		fm.delete(ctx.Value(FutureID).(Sequence))
	})

	fm.futures[id] = future
	return future
}

func (fm *futureManagerImpl) Get(id Sequence) Future {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	return fm.futures[id]
}

func (fm *futureManagerImpl) delete(id Sequence) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	delete(fm.futures, id)
}
