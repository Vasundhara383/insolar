// Code generated by "stringer -type=PacketType"; DO NOT EDIT.

package types

import "strconv"

const _PacketType_name = "TypePingTypeStoreTypeFindHostTypeFindValueTypeRPCTypeRelayTypeAuthenticationTypeCheckOriginTypeObtainIPTypeRelayOwnershipTypeKnownOuterHostsTypeCheckNodePrivTypeCascadeSendTypePulseTypeGetRandomHostsTypeGetNonceTypeCheckSignedNonceTypeExchangeUnsyncListsTypeExchangeUnsyncHashTypeDisconnectPingRPCCascadePulseGetRandomHostsBootstrapGetNonceAuthorizeDisconnect"

var _PacketType_index = [...]uint16{0, 8, 17, 29, 42, 49, 58, 76, 91, 103, 121, 140, 157, 172, 181, 199, 211, 231, 254, 276, 290, 294, 297, 304, 309, 323, 332, 340, 349, 359}

func (i PacketType) String() string {
	i -= 1
	if i < 0 || i >= PacketType(len(_PacketType_index)-1) {
		return "PacketType(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _PacketType_name[_PacketType_index[i]:_PacketType_index[i+1]]
}