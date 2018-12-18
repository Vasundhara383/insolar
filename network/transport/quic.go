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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"

	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/network/transport/relay"
	"github.com/lucas-clemente/quic-go"
	"github.com/pkg/errors"
)

type quicConnection struct {
	session quic.Session
	stream  quic.Stream
}

type quicTransport struct {
	baseTransport
	l           quic.Listener
	conn        net.PacketConn
	connections map[string]quicConnection
}

func newQuicTransport(conn net.PacketConn, proxy relay.Proxy, publicAddress string) (*quicTransport, error) {
	transport := &quicTransport{
		baseTransport: newBaseTransport(proxy, publicAddress),
		conn:          conn,
		connections:   make(map[string]quicConnection),
	}

	transport.sendFunc = transport.send
	return transport, nil
}

func (t *quicTransport) send(recvAddress string, data []byte) error {
	conn, ok := t.connections[recvAddress]
	var stream quic.Stream
	var err error
	if !ok {
		var session quic.Session
		session, stream, err = createConnection(recvAddress)
		if err != nil {
			return errors.Wrap(err, "[ send ] failed to create a connection")
		}
		t.connections[recvAddress] = quicConnection{session, stream}
	} else {
		stream = conn.stream
	}

	n, err := stream.Write(data)
	if err != nil {
		return errors.Wrap(err, "[ send ] failed to write to a stream")
	}

	if n != len(data) {
		return errors.New("[ send ] sent a part of data")
	}

	return nil
}

// Start starts networking.
func (t *quicTransport) Listen(ctx context.Context) error {
	t.mutex.Lock()

	listener, err := quic.Listen(t.conn, generateTLSConfig(), nil)
	if err != nil {
		t.mutex.Unlock()

		return err
	}

	t.l = listener

	log.Debug("Start QUIC transport")
	t.prepareListen()

	t.mutex.Unlock()

	for {
		session, err := t.l.Accept()
		if err != nil {
			<-t.disconnectFinished
			return err
		}

		log.Debugf("accept from: %s", session.RemoteAddr().String())
		go t.handleAcceptedConnection(session)
	}
}

// Stop stops networking.
func (t *quicTransport) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	log.Debug("[ Stop ] Stop QUIC transport")
	t.prepareDisconnect()

	err := t.l.Close()
	if err != nil {
		log.Errorln("[ Stop ] Failed to close socket:", err.Error())
	}

	err = t.closeConnections()
	if err != nil {
		log.Error(err, "[ Stop ] failed to close sessions")
	}
	err = t.conn.Close()
	if err != nil {
		log.Error(err, "[ Stop ] failed to close a connection")
	}
}

func (t *quicTransport) handleAcceptedConnection(session quic.Session) {
	stream, err := session.AcceptStream()
	if err != nil {
		log.Error(err, "[ handleAcceptedConnection ] failed to get a stream")
	}

	msg, err := t.serializer.DeserializePacket(stream)
	if err != nil {
		log.Error(err, "[ handleAcceptedConnection ] failed to deserialize a packet")
	}

	go t.handlePacket(msg)

	err = stream.Close()
	if err != nil {
		log.Error(err, "[ handleAcceptedConnection ] failed to close a stream")
	}
}

func (t *quicTransport) closeConnections() error {
	var err error
	for _, conn := range t.connections {
		err = conn.stream.Close()
		if err != nil {
			return errors.Wrap(err, "[ closeConnections ] failed to close a stream")
		}
		err = conn.session.Close()
		if err != nil {
			return errors.Wrap(err, "[ closeConnections ] failed to close a session")
		}
	}
	return nil
}

func createConnection(addr string) (quic.Session, quic.Stream, error) {
	// TODO: NETD18-78
	session, err := quic.DialAddr(addr, &tls.Config{InsecureSkipVerify: true}, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "[ createConnection ] failed to create a session")
	}
	stream, err := session.OpenStreamSync()
	if err != nil {
		return nil, nil, errors.Wrap(err, "[ createConnection ] failed to open a stream")
	}
	log.Debug("connected to: %s", session.RemoteAddr().String())
	return session, stream, nil
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}