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

package rootdomain

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/insolar/insolar/application/proxy/member"
	"github.com/insolar/insolar/application/proxy/nodedomain"
	"github.com/insolar/insolar/application/proxy/wallet"
	cryptoHelper "github.com/insolar/insolar/cryptohelpers/ecdsa"
	"github.com/insolar/insolar/networkcoordinator"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
)

// RootDomain is smart contract representing entrance point to system
type RootDomain struct {
	foundation.BaseContract
	RootMember    core.RecordRef
	NodeDomainRef core.RecordRef
}

// RegisterNode processes register node request
func (rd *RootDomain) RegisterNode(publicKey string, numberOfBootstrapNodes int, majorityRule int, roles []string, ip string) ([]byte, error) {
	domainRefs, err := rd.GetChildrenTyped(nodedomain.ClassReference)
	if err != nil {
		return nil, err
	}

	if len(domainRefs) == 0 {
		return nil, errors.New("No NodeDomain references")
	}
	nd := nodedomain.GetObject(domainRefs[0])

	cert, err := nd.RegisterNode(publicKey, numberOfBootstrapNodes, majorityRule, roles, ip)
	if err != nil {
		return nil, fmt.Errorf("Problems with RegisterNode: " + err.Error())
	}

	return cert, nil
}

func makeSeed() []byte {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	return seed
}

// Authorize checks is node authorized ( It's temporary method. Remove it when we have good tests )
func (rd *RootDomain) Authorize() (string, []core.NodeRole, error) {
	privateKey, err := cryptoHelper.GeneratePrivateKey()
	if err != nil {
		panic(err)
	}

	// Make signature
	seed := makeSeed()
	signature, err := cryptoHelper.Sign(seed, privateKey)
	if err != nil {
		panic(err)
	}

	// Register node
	serPubKey, err := cryptoHelper.ExportPublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(err)
	}

	rawJSON, err := rd.RegisterNode(serPubKey, 0, 0, []string{"virtual"}, "127.0.0.1")
	if err != nil {
		panic(err)
	}

	nodeRef, err := networkcoordinator.ExtractNodeRef(rawJSON)
	if err != nil {
		panic(err)
	}

	// Validate
	domainRefs, err := rd.GetChildrenTyped(nodedomain.ClassReference)
	if err != nil {
		panic(err)
	}
	nd := nodedomain.GetObject(domainRefs[0])

	return nd.Authorize(core.NewRefFromBase58(nodeRef), seed, signature)
}

// CreateMember processes create member request
func (rd *RootDomain) CreateMember(name string, key string) (string, error) {
	//if rd.GetContext().Caller != nil && *rd.GetContext().Caller == *rd.RootMember {
	memberHolder := member.New(name, key)
	m, err := memberHolder.AsChild(rd.GetReference())
	if err != nil {
		return "", err
	}

	wHolder := wallet.New(1000)
	_, err = wHolder.AsDelegate(m.GetReference())
	if err != nil {
		return "", err
	}

	return m.GetReference().String(), nil
	//}
	//return ""
}

// GetBalance processes get balance request
func (rd *RootDomain) GetBalance(reference string) (uint, error) {
	w, err := wallet.GetImplementationFrom(core.NewRefFromBase58(reference))
	if err != nil {
		return 0, err
	}

	return w.GetTotalBalance()
}

// SendMoney processes send money request
func (rd *RootDomain) SendMoney(from string, to string, amount uint) (bool, error) {
	walletFrom, err := wallet.GetImplementationFrom(core.NewRefFromBase58(from))
	if err != nil {
		return false, err
	}

	v := core.NewRefFromBase58(to)
	walletFrom.Transfer(amount, &v)
	return true, nil
}

func (rd *RootDomain) getUserInfoMap(m *member.Member) (map[string]interface{}, error) {
	w, err := wallet.GetImplementationFrom(m.GetReference())
	if err != nil {
		return nil, err
	}

	name, err := m.GetName()
	if err != nil {
		return nil, err
	}

	balance, err := w.GetTotalBalance()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"member": name,
		"wallet": balance,
	}, nil
}

// DumpUserInfo processes dump user info request
func (rd *RootDomain) DumpUserInfo(reference string) ([]byte, error) {
	m := member.GetObject(core.NewRefFromBase58(reference))

	res, err := rd.getUserInfoMap(m)
	if err != nil {
		return nil, err
	}

	return json.Marshal(res)
}

// DumpAllUsers processes dump all users request
func (rd *RootDomain) DumpAllUsers() ([]byte, error) {
	res := []map[string]interface{}{}
	crefs, err := rd.GetChildrenTyped(member.ClassReference)
	if err != nil {
		return nil, err
	}
	for _, cref := range crefs {
		m := member.GetObject(cref)
		userInfo, err := rd.getUserInfoMap(m)
		if err != nil {
			return nil, err
		}
		res = append(res, userInfo)
	}
	resJSON, _ := json.Marshal(res)
	return resJSON, nil
}

// GetNodeDomainRef returns reference of NodeDomain instance
func (rd *RootDomain) GetNodeDomainRef() (core.RecordRef, error) {
	return rd.NodeDomainRef, nil
}

// NewRootDomain creates new RootDomain
func NewRootDomain() (*RootDomain, error) {
	return &RootDomain{}, nil
}