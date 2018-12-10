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

package networkcoordinator

import (
	"context"

	"github.com/insolar/insolar/core"
)

// NetworkCoordinator encapsulates logic of network configuration
type NetworkCoordinator struct {
	//<<<<<<< HEAD
	// 	CertificateManager core.CertificateManager  `inject:""`
	// NetworkSwitcher    core.NetworkSwitcher     `inject:""`
	// ContractRequester  core.ContractRequester   `inject:""`
	MessageBus core.MessageBus `inject:""`
	// CS                 core.CryptographyService `inject:""`
	//
	realCoordinator Coordinator
	zeroCoordinator Coordinator
	// =======
	CertificateManager  core.CertificateManager  `inject:""`
	NetworkSwitcher     core.NetworkSwitcher     `inject:""`
	ContractRequester   core.ContractRequester   `inject:""`
	GenesisDataProvider core.GenesisDataProvider `inject:""`
	//Bus                 core.MessageBus          `inject:""`
	CS core.CryptographyService `inject:""`
	PM core.PulseManager        `inject:""`

	// realCoordinator core.NetworkCoordinator
	// zeroCoordinator core.NetworkCoordinator
	// >>>>>>> 802c42a26e8cae111d9322f5ba6e592b181e3a69
}

// New creates new NetworkCoordinator
func New() (*NetworkCoordinator, error) {
	return &NetworkCoordinator{}, nil
}

// <<<<<<< HEAD
// Init implements interface of Component
// func (nc *NetworkCoordinator) Init(ctx context.Context) error {
// 	nc.zeroCoordinator = newZeroNetworkCoordinator()
// 	nc.realCoordinator = newRealNetworkCoordinator()
// 	return nc.Bus.Register(core.TypeNodeSignRequest, nc.SignNode)
// =======
// Start implements interface of Component
func (nc *NetworkCoordinator) Start(ctx context.Context) error {
	nc.MessageBus.MustRegister(core.TypeNodeSignRequest, nc.signCertHandler)

	nc.zeroCoordinator = newZeroNetworkCoordinator()
	nc.realCoordinator = newRealNetworkCoordinator(
		nc.CertificateManager,
		nc.ContractRequester,
		nc.MessageBus,
		nc.CS,
	)
	return nil
	// >>>>>>> 5c81ba6a5ec63ab34f56b3cad4a2cd8432c41bd4
}

func (nc *NetworkCoordinator) getCoordinator() Coordinator {
	if nc.NetworkSwitcher.GetState() == core.CompleteNetworkState {
		return nc.realCoordinator
	}
	return nc.zeroCoordinator
}

// <<<<<<< HEAD
// GetCert method returns node certificate
//<<<<<<< HEAD
// func (nc *NetworkCoordinator) GetCert(ctx context.Context, nodeRef core.RecordRef) (core.Certificate, error) {
// 	res, err := nc.ContractRequester.SendRequest(ctx, &nodeRef, "GetNodeInfo", []interface{}{})
// 	if err != nil {
// 		return nil, errors.Wrap(err, "[ GetCert ] Couldn't call GetNodeInfo")
// 	}
// 	pKey, role, err := extractor.NodeInfoResponse(res.(*reply.CallMethod).Result)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "[ GetCert ] Couldn't extract response")
// 	}
//
// 	currentNodeCert := nc.CertificateManager.GetCertificate()
// 	cert, err := nc.CertificateManager.NewUnsignedCertificate(pKey, role, nodeRef.String())
// 	if err != nil {
// 		return nil, errors.Wrap(err, "[ GetCert ] Couldn't create certificate")
// 	}
//
// 	for i, node := range currentNodeCert.GetDiscoveryNodes() {
// 		if node.GetNodeRef() == currentNodeCert.GetNodeRef() {
// 			sign, err := nc.signNode(ctx, node.GetNodeRef())
// 			if err != nil {
// 				return nil, err
// 			}
// 			cert.(*certificate.Certificate).BootstrapNodes[i].NodeSign = sign
// 		} else {
// 			msg := message.NodeSignPayload{
// 				NodeRef: &nodeRef,
// 			}
// 			opts := core.MessageSendOptions{
// 				Receiver: node.GetNodeRef(),
// 			}
// 			r, err := nc.Bus.Send(ctx, &msg, &opts)
// 			if err != nil {
// 				return nil, err
// 			}
// 			sign := r.(reply.NodeSignInt).GetSign()
// 			cert.(*certificate.Certificate).BootstrapNodes[i].NodeSign = sign
// 		}
// 	}
// 	return cert, nil
// =======
// GetCert method returns node certificate by requesting sign from discovery nodes
func (nc *NetworkCoordinator) GetCert(ctx context.Context, registeredNodeRef *core.RecordRef) (core.Certificate, error) {
	return nc.getCoordinator().GetCert(ctx, registeredNodeRef)
	// >>>>>>> 5c81ba6a5ec63ab34f56b3cad4a2cd8432c41bd4
	//=======
	// func (nc *NetworkCoordinator) GetCert(ctx context.Context, nodeRef core.RecordRef) (core.Certificate, error) {
	// 	res, err := nc.ContractRequester.SendRequest(ctx, &nodeRef, "GetNodeInfo", []interface{}{})
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "[ GetCert ] Couldn't call GetNodeInfo")
	// 	}
	// 	pKey, role, err := extractor.NodeInfoResponse(res.(*reply.CallMethod).Result)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "[ GetCert ] Couldn't extract response")
	// 	}
	//
	// 	currentNodeCert := nc.CertificateManager.GetCertificate()
	// 	cert, err := nc.CertificateManager.NewUnsignedCertificate(pKey, role, nodeRef.String())
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "[ GetCert ] Couldn't create certificate")
	// 	}
	//
	// 	for i, node := range currentNodeCert.GetDiscoveryNodes() {
	// 		if node.GetNodeRef() == currentNodeCert.GetNodeRef() {
	// 			sign, err := nc.signNode(ctx, node.GetNodeRef())
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			currentNodeCert.(*certificate.Certificate).BootstrapNodes[i].NodeSign = sign
	// 		} else {
	// 			msg := message.NodeSignPayload{
	// 				NodeRef: &nodeRef,
	// 			}
	// 			opts := core.MessageSendOptions{
	// 				Receiver: node.GetNodeRef(),
	// 			}
	//
	// 			currentPulse, err := nc.PM.Current(ctx)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	//
	// 			r, err := nc.Bus.Send(ctx, &msg, *currentPulse, &opts)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			sign := r.(reply.NodeSignInt).GetSign()
	// 			currentNodeCert.(*certificate.Certificate).BootstrapNodes[i].NodeSign = sign
	// 		}
	// 	}
	// 	return cert, nil
	// >>>>>>> 802c42a26e8cae111d9322f5ba6e592b181e3a69
}

// ValidateCert validates node certificate
func (nc *NetworkCoordinator) ValidateCert(ctx context.Context, certificate core.AuthorizationCertificate) (bool, error) {
	return nc.CertificateManager.VerifyAuthorizationCertificate(certificate)
}

// signCertHandler is MsgBus handler that signs certificate for some node with node own key
func (nc *NetworkCoordinator) signCertHandler(ctx context.Context, p core.Parcel) (core.Reply, error) {
	return nc.getCoordinator().signCertHandler(ctx, p)
}

// WriteActiveNodes writes active nodes to ledger
func (nc *NetworkCoordinator) WriteActiveNodes(ctx context.Context, number core.PulseNumber, activeNodes []core.Node) error {
	return nc.getCoordinator().WriteActiveNodes(ctx, number, activeNodes)
}

// SetPulse writes pulse data on local storage
func (nc *NetworkCoordinator) SetPulse(ctx context.Context, pulse core.Pulse) error {
	return nc.getCoordinator().SetPulse(ctx, pulse)
}
