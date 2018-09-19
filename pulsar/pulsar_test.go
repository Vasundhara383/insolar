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

package pulsar

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net"
	"testing"

	"github.com/insolar/insolar/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockListener struct {
	mock.Mock
}

func (mock *mockListener) Accept() (net.Conn, error) {
	panic("implement me")
}

func (mock *mockListener) Close() error {
	panic("implement me")
}

func (mock *mockListener) Addr() net.Addr {
	panic("implement me")
}

func TestNewPulsar_WithoutNeighbours(t *testing.T) {
	assertObj := assert.New(t)
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	expectedPrivateKey, _ := ExportPrivateKey(privateKey)
	config := configuration.Pulsar{
		ConnectionType: "testType",
		ListenAddress:  "listedAddress",
		PrivateKey:     expectedPrivateKey,
	}
	actualConnectionType := ""
	actualAddress := ""

	result, err := NewPulsar(config, func(connectionType string, address string) (net.Listener, error) {
		actualConnectionType = connectionType
		actualAddress = address
		return &mockListener{}, nil
	})

	assertObj.NoError(err)
	parsedKey, _ := ImportPrivateKey(expectedPrivateKey)
	assertObj.Equal(parsedKey, result.PrivateKey)
	assertObj.Equal("testType", actualConnectionType)
	assertObj.Equal("listedAddress", actualAddress)
	assertObj.IsType(result.Sock, &mockListener{})
	assertObj.NotNil(result.PrivateKey)
}

func TestNewPulsar_WithNeighbours(t *testing.T) {
	assertObj := assert.New(t)

	firstPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	firstExpectedKey, _ := ExportPublicKey(&firstPrivateKey.PublicKey)

	secondPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	secondExpectedKey, _ := ExportPublicKey(&secondPrivateKey.PublicKey)

	expectedPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	parsedExpectedPrivateKey, _ := ExportPrivateKey(expectedPrivateKey)
	config := configuration.Pulsar{
		ConnectionType: "testType",
		ListenAddress:  "listedAddress",
		PrivateKey:     parsedExpectedPrivateKey,
		ListOfNeighbours: []*configuration.PulsarNodeAddress{
			{ConnectionType: "tcp", Address: "first", PublicKey: firstExpectedKey},
			{ConnectionType: "pct", Address: "second", PublicKey: secondExpectedKey},
		},
	}

	result, err := NewPulsar(config, func(connectionType string, address string) (net.Listener, error) {
		return &mockListener{}, nil
	})

	assertObj.NoError(err)
	assertObj.Equal(2, len(result.Neighbours))
	assertObj.Equal("tcp", result.Neighbours[firstExpectedKey].ConnectionType.String())
	assertObj.Equal("pct", result.Neighbours[secondExpectedKey].ConnectionType.String())
}

func TestSingAndVerify(t *testing.T) {
	assertObj := assert.New(t)
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	publicKey, _ := ExportPublicKey(&privateKey.PublicKey)

	signature, err := singData(privateKey, "This is the message to be signed and verified!")
	assertObj.NoError(err)

	checkSignature, err := checkSignature(&Payload{PublicKey: publicKey, Signature: signature, Body: "This is the message to be signed and verified!"})

	assertObj.Equal(true, checkSignature)
}