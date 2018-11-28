package artifactmanager

import (
	"testing"

	"github.com/gojuno/minimock"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/delegationtoken"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/ledger/index"
	"github.com/insolar/insolar/ledger/storage"
	"github.com/insolar/insolar/ledger/storage/storagetest"
	"github.com/insolar/insolar/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageHandler_HandleGetObject_Redirects(t *testing.T) {
	t.Parallel()
	ctx := inslogger.TestContext(t)
	mc := minimock.NewController(t)
	db, cleaner := storagetest.TmpDB(ctx, t)
	defer cleaner()
	defer mc.Finish()

	tf := testutils.NewDelegationTokenFactoryMock(mc)
	tf.IssueGetObjectRedirectMock.Return(&delegationtoken.GetObjectRedirect{Signature: []byte{1, 2, 3}}, nil)
	jc := testutils.NewJetCoordinatorMock(mc)
	h := NewMessageHandler(db, storage.NewRecentStorage(0), &configuration.ArtifactManager{
		LightChainLimit: 3,
	})
	h.JetCoordinator = jc
	h.DelegationTokenFactory = tf

	msg := message.GetObject{
		Head: *genRandomRef(0),
	}

	t.Run("redirects to heavy when no index", func(t *testing.T) {
		heavyRef := genRandomRef(0)
		jc.QueryRoleMock.Expect(
			ctx, core.RoleHeavyExecutor, &msg.Head, 0,
		).Return(
			[]core.RecordRef{*heavyRef}, nil,
		)
		rep, err := h.handleGetObject(ctx, 0, &message.Parcel{
			Msg: &msg,
		})
		require.NoError(t, err)
		redirect, ok := rep.(*reply.GetObjectRedirectReply)
		require.True(t, ok)
		token, ok := redirect.Token.(*delegationtoken.GetObjectRedirect)
		assert.Equal(t, []byte{1, 2, 3}, token.Signature)
		assert.Equal(t, heavyRef, redirect.GetReceiver())
		assert.Nil(t, redirect.StateID)
	})

	t.Run("redirect to light when has index and state later than limit", func(t *testing.T) {
		lightRef := genRandomRef(0)
		stateID := genRandomID(0)
		err := db.SetObjectIndex(ctx, msg.Head.Record(), &index.ObjectLifeline{
			LatestState: stateID,
		})
		require.NoError(t, err)
		jc.QueryRoleMock.Expect(
			ctx, core.RoleLightExecutor, &msg.Head, 0,
		).Return(
			[]core.RecordRef{*lightRef}, nil,
		)
		rep, err := h.handleGetObject(ctx, 1, &message.Parcel{
			Msg: &msg,
		})
		require.NoError(t, err)
		redirect, ok := rep.(*reply.GetObjectRedirectReply)
		require.True(t, ok)
		token, ok := redirect.Token.(*delegationtoken.GetObjectRedirect)
		assert.Equal(t, []byte{1, 2, 3}, token.Signature)
		assert.Equal(t, lightRef, redirect.GetReceiver())
		assert.Equal(t, stateID, redirect.StateID)
	})

	t.Run("redirect to heavy when has index and state earlier than limit", func(t *testing.T) {
		heavyRef := genRandomRef(0)
		stateID := genRandomID(0)

		err := db.SetObjectIndex(ctx, msg.Head.Record(), &index.ObjectLifeline{
			LatestState: stateID,
		})
		require.NoError(t, err)
		jc.QueryRoleMock.Expect(
			ctx, core.RoleHeavyExecutor, &msg.Head, 0,
		).Return(
			[]core.RecordRef{*heavyRef}, nil,
		)
		rep, err := h.handleGetObject(ctx, 5, &message.Parcel{
			Msg: &msg,
		})
		require.NoError(t, err)
		redirect, ok := rep.(*reply.GetObjectRedirectReply)
		require.True(t, ok)
		token, ok := redirect.Token.(*delegationtoken.GetObjectRedirect)
		assert.Equal(t, []byte{1, 2, 3}, token.Signature)
		assert.Equal(t, heavyRef, redirect.GetReceiver())
		assert.Equal(t, stateID, redirect.StateID)
	})
}

func TestMessageHandler_HandleGetChildren_Redirects(t *testing.T) {
	t.Parallel()
	ctx := inslogger.TestContext(t)
	mc := minimock.NewController(t)
	db, cleaner := storagetest.TmpDB(ctx, t)
	defer cleaner()
	defer mc.Finish()

	tf := testutils.NewDelegationTokenFactoryMock(mc)
	tf.IssueGetChildrenRedirectMock.Return(&delegationtoken.GetChildrenRedirect{Signature: []byte{1, 2, 3}}, nil)
	jc := testutils.NewJetCoordinatorMock(mc)
	h := NewMessageHandler(db, storage.NewRecentStorage(0), &configuration.ArtifactManager{
		LightChainLimit: 3,
	})
	h.JetCoordinator = jc
	h.DelegationTokenFactory = tf

	msg := message.GetChildren{
		Parent: *genRandomRef(0),
	}

	t.Run("redirects to heavy when no index", func(t *testing.T) {
		heavyRef := genRandomRef(0)
		jc.QueryRoleMock.Expect(
			ctx, core.RoleHeavyExecutor, &msg.Parent, 0,
		).Return(
			[]core.RecordRef{*heavyRef}, nil,
		)
		rep, err := h.handleGetChildren(ctx, 0, &message.Parcel{
			Msg: &msg,
		})
		require.NoError(t, err)
		redirect, ok := rep.(*reply.GetChildrenRedirect)
		require.True(t, ok)
		token, ok := redirect.Token.(*delegationtoken.GetChildrenRedirect)
		assert.Equal(t, []byte{1, 2, 3}, token.Signature)
		assert.Equal(t, heavyRef, redirect.GetReceiver())
	})

	t.Run("redirect to light when has index and child later than limit", func(t *testing.T) {
		lightRef := genRandomRef(0)
		err := db.SetObjectIndex(ctx, msg.Parent.Record(), &index.ObjectLifeline{
			ChildPointer: genRandomID(0),
		})
		require.NoError(t, err)
		jc.QueryRoleMock.Expect(
			ctx, core.RoleLightExecutor, &msg.Parent, 0,
		).Return(
			[]core.RecordRef{*lightRef}, nil,
		)
		rep, err := h.handleGetChildren(ctx, 1, &message.Parcel{
			Msg: &msg,
		})
		require.NoError(t, err)
		redirect, ok := rep.(*reply.GetChildrenRedirect)
		require.True(t, ok)
		token, ok := redirect.Token.(*delegationtoken.GetChildrenRedirect)
		assert.Equal(t, []byte{1, 2, 3}, token.Signature)
		assert.Equal(t, lightRef, redirect.GetReceiver())
	})

	t.Run("redirect to heavy when has index and child earlier than limit", func(t *testing.T) {
		heavyRef := genRandomRef(0)
		err := db.SetObjectIndex(ctx, msg.Parent.Record(), &index.ObjectLifeline{
			ChildPointer: genRandomID(0),
		})
		require.NoError(t, err)
		jc.QueryRoleMock.Expect(
			ctx, core.RoleHeavyExecutor, &msg.Parent, 0,
		).Return(
			[]core.RecordRef{*heavyRef}, nil,
		)
		rep, err := h.handleGetChildren(ctx, 5, &message.Parcel{
			Msg: &msg,
		})
		require.NoError(t, err)
		redirect, ok := rep.(*reply.GetChildrenRedirect)
		require.True(t, ok)
		token, ok := redirect.Token.(*delegationtoken.GetChildrenRedirect)
		assert.Equal(t, []byte{1, 2, 3}, token.Signature)
		assert.Equal(t, heavyRef, redirect.GetReceiver())
	})
}