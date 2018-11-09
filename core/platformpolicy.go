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

package core

import (
	"crypto"
	"hash"
)

type Hasher interface {
	hash.Hash

	Hash([]byte) []byte
}

type PlatformCryptographyScheme interface {
	ReferenceHasher() Hasher
	IntegrityHasher() Hasher

	Signer(crypto.PrivateKey) Signer
	Verifier(crypto.PublicKey) Verifier
}

type PlatformPolicy interface {
	CryptographyScheme() PlatformCryptographyScheme
}
