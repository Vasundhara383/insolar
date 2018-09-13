/*
 *    Copyright 2018 INS Ecosystem
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

package hostnetwork

import (
	"log"
	"time"

	"github.com/insolar/insolar/network/hostnetwork/hosthandler"
	"github.com/insolar/insolar/network/hostnetwork/packet"
	"github.com/pkg/errors"
)

// RelayRequest sends relay request to target.
func RelayRequest(hostHandler hosthandler.HostHandler, command, targetID string) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	var typedCommand packet.CommandType
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("RelayRequest: target for relay request not found")
		return err
	}

	switch command {
	case "start":
		typedCommand = packet.StartRelay
	case "stop":
		typedCommand = packet.StopRelay
	default:
		err = errors.New("RelayRequest: unknown command")
		return err
	}
	request := packet.NewRelayPacket(typedCommand, hostHandler.HtFromCtx(ctx).Origin, targetHost)
	future, err := hostHandler.SendRequest(request)

	if err != nil {
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			err = errors.New("RelayRequest: chanel closed unexpectedly")
			return err
		}

		response := rsp.Data.(*packet.ResponseRelay)
		err = handleRelayResponse(hostHandler, ctx, response, targetID)
		if err != nil {
			return err
		}

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("RelayRequest: timeout")
		return err
	}

	return nil
}

// CheckOriginRequest send a request to check target host originality
func CheckOriginRequest(hostHandler hosthandler.HostHandler, targetID string) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("CheckOriginRequest: target for relay request not found")
		return err
	}

	request := packet.NewCheckOriginPacket(hostHandler.HtFromCtx(ctx).Origin, targetHost)
	future, err := hostHandler.SendRequest(request)

	if err != nil {
		log.Println(err.Error())
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			err = errors.New("CheckOriginRequest: chanel closed unexpectedly")
			return err
		}

		response := rsp.Data.(*packet.ResponseCheckOrigin)
		handleCheckOriginResponse(hostHandler, response, targetID)

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("CheckOriginRequest: timeout")
		return err
	}

	return nil
}

// AuthenticationRequest sends an authentication request.
func AuthenticationRequest(hostHandler hosthandler.HostHandler, command, targetID string) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("AuthenticationRequest: target for auth request not found")
		return err
	}

	origin := hostHandler.HtFromCtx(ctx).Origin
	var authCommand packet.CommandType
	switch command {
	case "begin":
		authCommand = packet.BeginAuth
	case "revoke":
		authCommand = packet.RevokeAuth
	default:
		err = errors.New("AuthenticationRequest: unknown command")
		return err
	}
	request := packet.NewAuthPacket(authCommand, origin, targetHost)
	future, err := hostHandler.SendRequest(request)

	if err != nil {
		log.Println(err.Error())
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			err = errors.New("AuthenticationRequest: chanel closed unexpectedly")
			return err
		}

		response := rsp.Data.(*packet.ResponseAuth)
		err = handleAuthResponse(hostHandler, response, targetHost.ID.KeyString())
		if err != nil {
			return err
		}

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("AuthenticationRequest: timeout")
		return err
	}

	return nil
}

// ObtainIPRequest is request to self IP obtaining.
func ObtainIPRequest(hostHandler hosthandler.HostHandler, targetID string) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("ObtainIPRequest: target for relay request not found")
		return err
	}

	origin := hostHandler.HtFromCtx(ctx).Origin
	request := packet.NewObtainIPPacket(origin, targetHost)

	future, err := hostHandler.SendRequest(request)

	if err != nil {
		log.Println(err.Error())
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			err = errors.New("ObtainIPRequest: chanel closed unexpectedly")
			return err
		}

		response := rsp.Data.(*packet.ResponseObtainIP)
		err = handleObtainIPResponse(hostHandler, response, targetHost.ID.KeyString())
		if err != nil {
			return err
		}

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("ObtainIPRequest: timeout")
		return err
	}

	return nil
}

// RelayOwnershipRequest sends a relay ownership request.
func RelayOwnershipRequest(hostHandler hosthandler.HostHandler, targetID string) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	if err != nil {
		return err
	}
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("relayOwnershipRequest: target for relay request not found")
		return err
	}

	request := packet.NewRelayOwnershipPacket(hostHandler.HtFromCtx(ctx).Origin, targetHost, true)
	future, err := hostHandler.SendRequest(request)

	if err != nil {
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			return err
		}

		response := rsp.Data.(*packet.ResponseRelayOwnership)
		handleRelayOwnership(hostHandler, response, targetID)

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("relayOwnershipRequest: timeout")
		return err
	}

	return nil
}

func checkNodePrivRequest(hostHandler hosthandler.HostHandler, targetID string, roleKey string) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("checkNodePrivRequest: target for check node privileges request not found")
		return err
	}

	origin := hostHandler.HtFromCtx(ctx).Origin
	request := packet.NewCheckNodePrivPacket(origin, targetHost, roleKey)
	future, err := hostHandler.SendRequest(request)

	if err != nil {
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			err = errors.New("checkNodePrivRequest: chanel closed unexpectedly")
			return err
		}

		response := rsp.Data.(*packet.ResponseCheckNodePriv)
		err = handleCheckNodePrivResponse(hostHandler, response, roleKey)
		if err != nil {
			return err
		}

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("checkNodePrivRequest: timeout")
		return err
	}

	return nil
}

func knownOuterHostsRequest(hostHandler hosthandler.HostHandler, targetID string, hosts int) error {
	ctx, err := NewContextBuilder(hostHandler).SetDefaultHost().Build()
	if err != nil {
		return err
	}
	targetHost, exist, err := hostHandler.FindHost(ctx, targetID)
	if err != nil {
		return err
	}
	if !exist {
		err = errors.New("knownOuterHostsRequest: target for relay request not found")
		return err
	}

	request := packet.NewKnownOuterHostsPacket(hostHandler.HtFromCtx(ctx).Origin, targetHost, hosts)
	future, err := hostHandler.SendRequest(request)

	if err != nil {
		return err
	}

	select {
	case rsp := <-future.Result():
		if rsp == nil {
			return err
		}

		response := rsp.Data.(*packet.ResponseKnownOuterHosts)
		err = handleKnownOuterHosts(hostHandler, response, targetID)
		if err != nil {
			return err
		}

	case <-time.After(hostHandler.GetPacketTimeout()):
		future.Cancel()
		err = errors.New("knownOuterHostsRequest: timeout")
		return err
	}

	return nil
}
