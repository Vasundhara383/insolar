package organization

import (
	"encoding/json"
	"fmt"
	contractMember "github.com/insolar/insolar/application/contract/member"
	"github.com/insolar/insolar/application/noncontract/group"
	"github.com/insolar/insolar/application/noncontract/participant"
	proxyMember "github.com/insolar/insolar/application/proxy/member"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
)

type Organization struct {
	foundation.BaseContract
	group.Group
	Requisites string
}

func New(name string, key string, requisites string) (*Organization, error) {
	return &Organization{
		foundation.BaseContract{},
		group.Group{
			participant.Participant{name, key},
		},
		requisites}, nil
}

///////////////////impl/////////////////////
func (o *Organization) GetName() (string, error) {
	return o.Participant.GetName()
}

var INSATTR_GetPublicKey_API = true

func (o *Organization) GetPublicKey() (string, error) {
	return o.Participant.GetPublicKey()
}

///////////////////impl end//////////////////

func (o *Organization) GetRequisites() (string, error) {
	return o.Requisites, nil
}

func (o *Organization) VerifySig(method string, params []byte, seed []byte, sign []byte) error {
	args, err := core.MarshalArgs(o.GetReference(), method, params, seed)
	if err != nil {
		return fmt.Errorf("[ verifySig ] Can't MarshalArgs: %s", err.Error())
	}
	key, err := o.GetPublicKey()
	if err != nil {
		return fmt.Errorf("[ verifySig ]: %s", err.Error())
	}

	publicKey, err := foundation.ImportPublicKey(key)
	if err != nil {
		return fmt.Errorf("[ verifySig ] Invalid public key")
	}

	verified := foundation.Verify(args, sign, publicKey)
	if !verified {
		return fmt.Errorf("[ verifySig ] Incorrect signature")
	}
	return nil
}

// DumpAllOrganizationMembers processes dump all organization members
func (o *Organization) GetMembers() (resultJSON []byte, err error) {

	crefs, err := o.GetChildrenTyped(proxyMember.GetPrototype())
	if err != nil {
		return nil, fmt.Errorf("[ GetMembers ] Can't get children: %s", err.Error())
	}

	res := []contractMember.Member{}
	for _, cref := range crefs {
		m := proxyMember.GetObject(cref)

		memberJSON, err := m.ToOut()
		if err != nil {
			return nil, fmt.Errorf("[ GetMembers ] Problem with making request: %s", err.Error())
		}

		cMember := contractMember.Member{}
		err = json.Unmarshal(memberJSON, &cMember)
		if err != nil {
			return nil, fmt.Errorf("[ GetMembers ] Problem with unmarshal member from response: %s", err.Error())
		}

		res = append(res, cMember)
	}

	resultJSON, err = json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("[ GetMembers ] Problem with marshal members: %s", err.Error())
	}

	return resultJSON, nil
}