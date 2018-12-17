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

// Package logicrunner - infrastructure for executing smartcontracts
package logicrunner

import (
	"bytes"
	"context"
	"encoding/gob"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/logicrunner/builtin"
	"github.com/insolar/insolar/logicrunner/goplugin"
)

type Ref = core.RecordRef

// Context of one contract execution
type ObjectState struct {
	sync.Mutex
	Ref *Ref

	ExecutionState *ExecutionState
	Validation     *ExecutionState
	Consensus      *Consensus
}

type ExecutionState struct {
	sync.Mutex
	objectbody *ObjectBody
	deactivate bool
	nonce      uint64

	Behaviour ValidationBehaviour
	Current   *CurrentExecution
	Queue     []ExecutionQueueElement
}

type CurrentExecution struct {
	sync.Mutex
	Context      context.Context
	LogicContext *core.LogicCallContext
	Request      *Ref
}

type ExecutionQueueResult struct {
	reply        core.Reply
	err          error
	somebodyElse bool
}

type ExecutionQueueElement struct {
	ctx     context.Context
	parcel  core.Parcel
	request *Ref
	pulse   core.PulseNumber
	result  chan ExecutionQueueResult
}

type Error struct {
	Err      error
	Request  *Ref
	Contract *Ref
	Method   string
}

func (lre Error) Error() string {
	var buffer bytes.Buffer

	buffer.WriteString(lre.Err.Error())
	if lre.Contract != nil {
		buffer.WriteString(" Contract=" + lre.Contract.String())
	}
	if lre.Method != "" {
		buffer.WriteString(" Method=" + lre.Method)
	}
	if lre.Request != nil {
		buffer.WriteString(" Request=" + lre.Request.String())
	}

	return buffer.String()
}

func (os *ObjectState) MustModeState(mode string) (res *ExecutionState) {
	switch mode {
	case "execution":
		res = os.ExecutionState
	case "validation":
		res = os.Validation
	default:
		panic("'" + mode + "' is unknown object processing mode")
	}
	if res == nil {
		panic("object is not in " + mode + " mode")
	}
	return res
}

func (os *ObjectState) WrapError(err error, message string) error {
	if err == nil {
		err = errors.New(message)
	} else {
		err = errors.Wrap(err, message)
	}
	return Error{
		Err:      err,
		Contract: os.Ref,
	}
}

func (es *ExecutionState) WrapError(err error, message string) error {
	if err == nil {
		err = errors.New(message)
	} else {
		err = errors.Wrap(err, message)
	}
	res := Error{Err: err}
	if es.objectbody != nil {
		res.Contract = es.objectbody.objDescriptor.HeadRef()
	}
	if es.Current != nil {
		res.Request = es.Current.Request
	}
	return res
}

func (es *ExecutionState) ReleaseQueue() {
	es.Lock()
	defer es.Unlock()

	q := es.Queue
	es.Queue = make([]ExecutionQueueElement, 0)

	for _, qe := range q {
		qe.result <- ExecutionQueueResult{somebodyElse: true}
		close(qe.result)
	}
}

// LogicRunner is a general interface of contract executor
type LogicRunner struct {
	// FIXME: Ledger component is deprecated. Inject required sub-components.
	MessageBus                 core.MessageBus                 `inject:""`
	Ledger                     core.Ledger                     `inject:""`
	NodeNetwork                core.NodeNetwork                `inject:""`
	PlatformCryptographyScheme core.PlatformCryptographyScheme `inject:""`
	ParcelFactory              message.ParcelFactory           `inject:""`
	PulseManager               core.PulseManager               `inject:""`
	ArtifactManager            core.ArtifactManager            `inject:""`
	JetCoordinator             core.JetCoordinator             `inject:""`

	Executors    [core.MachineTypesLastID]core.MachineLogicExecutor
	machinePrefs []core.MachineType
	Cfg          *configuration.LogicRunner

	state      map[Ref]*ObjectState // if object exists, we are validating or executing it right now
	stateMutex sync.Mutex

	sock net.Listener
}

// NewLogicRunner is constructor for LogicRunner
func NewLogicRunner(cfg *configuration.LogicRunner) (*LogicRunner, error) {
	if cfg == nil {
		return nil, errors.New("LogicRunner have nil configuration")
	}
	res := LogicRunner{
		Cfg:   cfg,
		state: make(map[Ref]*ObjectState),
	}
	return &res, nil
}

// Start starts logic runner component
func (lr *LogicRunner) Start(ctx context.Context) error {
	if lr.Cfg.BuiltIn != nil {
		bi := builtin.NewBuiltIn(lr.MessageBus, lr.ArtifactManager)
		if err := lr.RegisterExecutor(core.MachineTypeBuiltin, bi); err != nil {
			return err
		}
		lr.machinePrefs = append(lr.machinePrefs, core.MachineTypeBuiltin)
	}

	if lr.Cfg.GoPlugin != nil {
		if lr.Cfg.RPCListen != "" {
			StartRPC(ctx, lr, lr.PulseManager)
		}

		gp, err := goplugin.NewGoPlugin(lr.Cfg, lr.MessageBus, lr.ArtifactManager)
		if err != nil {
			return err
		}
		if err := lr.RegisterExecutor(core.MachineTypeGoPlugin, gp); err != nil {
			return err
		}
		lr.machinePrefs = append(lr.machinePrefs, core.MachineTypeGoPlugin)
	}

	lr.RegisterHandlers()

	return nil
}

func (lr *LogicRunner) RegisterHandlers() {
	lr.MessageBus.MustRegister(core.TypeCallMethod, lr.Execute)
	lr.MessageBus.MustRegister(core.TypeCallConstructor, lr.Execute)
	lr.MessageBus.MustRegister(core.TypeExecutorResults, lr.ExecutorResults)
	lr.MessageBus.MustRegister(core.TypeValidateCaseBind, lr.ValidateCaseBind)
	lr.MessageBus.MustRegister(core.TypeValidationResults, lr.ProcessValidationResults)
}

// Stop stops logic runner component and its executors
func (lr *LogicRunner) Stop(ctx context.Context) error {
	reterr := error(nil)
	for _, e := range lr.Executors {
		if e == nil {
			continue
		}
		err := e.Stop()
		if err != nil {
			reterr = errors.Wrap(reterr, err.Error())
		}
	}

	if lr.sock != nil {
		if err := lr.sock.Close(); err != nil {
			return err
		}
	}

	return reterr
}

func (lr *LogicRunner) CheckOurRole(ctx context.Context, msg core.Message, role core.DynamicRole) error {
	// TODO do map of supported objects for pulse, go to jetCoordinator only if map is empty for ref
	target := msg.DefaultTarget()
	isAuthorized, err := lr.JetCoordinator.IsAuthorized(
		ctx, role, target.Record(), lr.pulse(ctx).PulseNumber, lr.NodeNetwork.GetOrigin().ID(),
	)
	if err != nil {
		return errors.Wrap(err, "authorization failed with error")
	}
	if !isAuthorized {
		return errors.New("can't execute this object")
	}
	return nil
}

func (lr *LogicRunner) RegisterRequest(ctx context.Context, parcel core.Parcel) (*Ref, error) {
	id, err := lr.ArtifactManager.RegisterRequest(ctx, parcel)
	if err != nil {
		return nil, err
	}

	// TODO: use proper conversion
	res := &Ref{}
	res.SetRecord(*id)
	return res, nil
}

// Execute runs a method on an object, ATM just thin proxy to `GoPlugin.Exec`
func (lr *LogicRunner) Execute(ctx context.Context, parcel core.Parcel) (core.Reply, error) {
	msg, ok := parcel.Message().(message.IBaseLogicMessage)
	if !ok {
		return nil, errors.New("Execute( ! message.IBaseLogicMessage )")
	}
	ref := msg.GetReference()

	err := lr.CheckOurRole(ctx, msg, core.DynamicRoleVirtualExecutor)
	if err != nil {
		return nil, errors.Wrap(err, "can't play role")
	}

	os := lr.UpsertObjectState(ref)

	os.Lock()
	if os.ExecutionState == nil {
		os.ExecutionState = &ExecutionState{
			Queue:     make([]ExecutionQueueElement, 0),
			Behaviour: &ValidationSaver{lr: lr, caseBind: NewCaseBind()},
		}
	}
	es := os.ExecutionState
	os.Unlock()

	es.Lock()
	if es.Current != nil {
		// TODO: check no wait call
		if inslogger.TraceID(es.Current.Context) == inslogger.TraceID(ctx) {
			es.Unlock()
			return nil, os.WrapError(nil, "loop detected")
		}
	}
	es.Unlock()

	request, err := lr.RegisterRequest(ctx, parcel)
	if err != nil {
		return nil, os.WrapError(err, "can't create request")
	}

	qElement := ExecutionQueueElement{
		ctx:     ctx,
		parcel:  parcel,
		request: request,
		pulse:   lr.pulse(ctx).PulseNumber,
		result:  make(chan ExecutionQueueResult, 1),
	}

	es.Lock()
	es.Queue = append(es.Queue, qElement)

	if es.Current == nil {
		inslogger.FromContext(ctx).Debug("Starting a new queue processor")
		go lr.ProcessExecutionQueue(ctx, es)
	}
	es.Unlock()

	if msg, ok := parcel.Message().(*message.CallMethod); ok && msg.ReturnMode == message.ReturnNoWait {
		return &reply.CallMethod{}, nil
	}

	res := <-qElement.result
	if res.err != nil {
		return nil, res.err
	} else if res.somebodyElse {
		panic("not implemented, should be implemented as part of async contract calls")
	}
	return res.reply, nil
}

func (lr *LogicRunner) ProcessExecutionQueue(ctx context.Context, es *ExecutionState) {
	// Current == nil indicates that we have no queue processor
	// and one should be started
	defer func() {
		inslogger.FromContext(ctx).Debug("Quiting queue processing, empty")
		es.Lock()
		es.Current = nil
		es.Unlock()
	}()

	for {
		es.Lock()
		q := es.Queue
		if len(q) == 0 {
			es.Unlock()
			return
		}
		qe, q := q[0], q[1:]
		es.Queue = q

		es.Current = &CurrentExecution{
			Request: qe.request,
		}
		es.Unlock()

		res := ExecutionQueueResult{}

		finish := func() {
			qe.result <- res
			close(qe.result)
		}

		recordingBus, err := lr.MessageBus.NewRecorder(qe.ctx, *lr.pulse(qe.ctx))
		if err != nil {
			res.err = err
			finish()
			continue
		}

		es.Current.Context = core.ContextWithMessageBus(qe.ctx, recordingBus)

		inslogger.FromContext(qe.ctx).Debug("Registering request within execution behaviour")
		es.Behaviour.(*ValidationSaver).NewRequest(
			qe.parcel.Message(), *qe.request, recordingBus,
		)

		res.reply, res.err = lr.executeOrValidate(es.Current.Context, es, qe.parcel)
		// TODO: check pulse change and do different things

		inslogger.FromContext(qe.ctx).Debug("Registering result within execution behaviour")
		err = es.Behaviour.Result(res.reply, res.err)
		if err != nil {
			res.err = err
		}

		finish()
	}
}

func (lr *LogicRunner) executeOrValidate(
	ctx context.Context, es *ExecutionState, parcel core.Parcel,
) (
	core.Reply, error,
) {
	msg := parcel.Message().(message.IBaseLogicMessage)
	ref := msg.GetReference()

	es.Current.LogicContext = &core.LogicCallContext{
		Mode:            es.Behaviour.Mode(),
		Caller:          msg.GetCaller(),
		Callee:          &ref,
		Request:         es.Current.Request,
		Time:            time.Now(), // TODO: probably we should take it earlier
		Pulse:           *lr.pulse(ctx),
		TraceID:         inslogger.TraceID(ctx),
		CallerPrototype: msg.GetCallerPrototype(),
	}

	switch m := msg.(type) {
	case *message.CallMethod:
		re, err := lr.executeMethodCall(ctx, es, m)
		return re, err

	case *message.CallConstructor:
		re, err := lr.executeConstructorCall(ctx, es, m)
		return re, err

	default:
		panic("Unknown e type")
	}
}

// ObjectBody is an inner representation of object and all it accessory
// make it private again when we start it serialize before sending
type ObjectBody struct {
	objDescriptor   core.ObjectDescriptor
	Object          []byte
	Prototype       *Ref
	CodeMachineType core.MachineType
	CodeRef         *Ref
	Parent          *Ref
}

func init() {
	gob.Register(&ObjectBody{})
}

func (lr *LogicRunner) executeMethodCall(ctx context.Context, es *ExecutionState, m *message.CallMethod) (core.Reply, error) {
	if es.objectbody == nil {
		objDesc, protoDesc, codeDesc, err := lr.getDescriptorsByObjectRef(ctx, m.ObjectRef)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get descriptors by object reference")
		}
		es.objectbody = &ObjectBody{
			objDescriptor:   objDesc,
			Object:          objDesc.Memory(),
			Prototype:       protoDesc.HeadRef(),
			CodeMachineType: codeDesc.MachineType(),
			CodeRef:         codeDesc.Ref(),
			Parent:          objDesc.Parent(),
		}
		inslogger.FromContext(ctx).Info("LogicRunner.executeMethodCall starts")
	}

	es.Current.LogicContext.Prototype = es.objectbody.Prototype
	es.Current.LogicContext.Code = es.objectbody.CodeRef
	es.Current.LogicContext.Parent = es.objectbody.Parent
	// it's needed to assure that we call method on ref, that has same prototype as proxy, that we import in contract code
	if !m.ProxyPrototype.IsEmpty() && !m.ProxyPrototype.Equal(*es.objectbody.Prototype) {
		return nil, errors.New("proxy call error: try to call method of prototype as method of another prototype")
	}

	executor, err := lr.GetExecutor(es.objectbody.CodeMachineType)
	if err != nil {
		return nil, es.WrapError(err, "no executor registered")
	}

	newData, result, err := executor.CallMethod(
		ctx, es.Current.LogicContext, *es.objectbody.CodeRef, es.objectbody.Object, m.Method, m.Arguments,
	)
	if err != nil {
		return nil, es.WrapError(err, "executor error")
	}

	am := lr.ArtifactManager
	if es.deactivate {
		_, err = am.DeactivateObject(
			ctx, Ref{}, *es.Current.Request, es.objectbody.objDescriptor,
		)
	} else {
		od, e := am.UpdateObject(ctx, Ref{}, *es.Current.Request, es.objectbody.objDescriptor, newData)
		err = e
		if od != nil && e == nil {
			es.objectbody.objDescriptor = od
		}
	}
	if err != nil {
		return nil, es.WrapError(err, "couldn't update object")
	}
	_, err = am.RegisterResult(ctx, *es.Current.Request, result)
	if err != nil {
		return nil, es.WrapError(err, "couldn't save results")
	}

	es.objectbody.Object = newData

	return &reply.CallMethod{Data: newData, Result: result}, nil
}

func (lr *LogicRunner) getDescriptorsByPrototypeRef(
	ctx context.Context, protoRef Ref,
) (
	core.ObjectDescriptor, core.CodeDescriptor, error,
) {
	protoDesc, err := lr.ArtifactManager.GetObject(ctx, protoRef, nil, false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't get prototype descriptor")
	}
	codeRef, err := protoDesc.Code()
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't get code reference")
	}
	// we don't want to record GetCode messages because of cache
	ctx = core.ContextWithMessageBus(ctx, lr.MessageBus)
	codeDesc, err := lr.ArtifactManager.GetCode(ctx, *codeRef)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't get code descriptor")
	}

	return protoDesc, codeDesc, nil
}

func (lr *LogicRunner) getDescriptorsByObjectRef(
	ctx context.Context, objRef Ref,
) (
	core.ObjectDescriptor, core.ObjectDescriptor, core.CodeDescriptor, error,
) {
	objDesc, err := lr.ArtifactManager.GetObject(ctx, objRef, nil, false)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "couldn't get object")
	}

	protoRef, err := objDesc.Prototype()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "couldn't get prototype reference")
	}

	protoDesc, codeDesc, err := lr.getDescriptorsByPrototypeRef(ctx, *protoRef)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "couldn't resolve prototype reference to descriptors")
	}

	return objDesc, protoDesc, codeDesc, nil
}

func (lr *LogicRunner) executeConstructorCall(
	ctx context.Context, es *ExecutionState, m *message.CallConstructor,
) (
	core.Reply, error,
) {
	if es.Current.LogicContext.Caller.IsEmpty() {
		return nil, es.WrapError(nil, "Call constructor from nowhere")
	}

	protoDesc, codeDesc, err := lr.getDescriptorsByPrototypeRef(ctx, m.PrototypeRef)
	if err != nil {
		return nil, es.WrapError(err, "couldn't descriptors")
	}
	es.Current.LogicContext.Prototype = protoDesc.HeadRef()
	es.Current.LogicContext.Code = codeDesc.Ref()

	executor, err := lr.GetExecutor(codeDesc.MachineType())
	if err != nil {
		return nil, es.WrapError(err, "no executer registered")
	}

	newData, err := executor.CallConstructor(ctx, es.Current.LogicContext, *codeDesc.Ref(), m.Name, m.Arguments)
	if err != nil {
		return nil, es.WrapError(err, "executer error")
	}

	switch m.SaveAs {
	case message.Child, message.Delegate:
		_, err = lr.ArtifactManager.ActivateObject(
			ctx,
			Ref{}, *es.Current.Request, m.ParentRef, m.PrototypeRef, m.SaveAs == message.Delegate, newData,
		)
		return &reply.CallConstructor{Object: es.Current.Request}, err
	default:
		return nil, es.WrapError(nil, "unsupported type of save object")
	}
}

func (lr *LogicRunner) OnPulse(ctx context.Context, pulse core.Pulse) error {
	// start of new Pulse, lock CaseBind data, copy it, clean original, unlock original

	lr.stateMutex.Lock()
	defer lr.stateMutex.Unlock()

	messages := make([]core.Message, 0)

	// send copy for validation
	for ref, state := range lr.state {
		state.RefreshConsensus()

		if es := state.ExecutionState; es != nil {
			caseBind := es.Behaviour.(*ValidationSaver).caseBind
			messages = append(
				messages,
				caseBind.ToValidateMessage(ctx, ref, pulse),
				caseBind.ToExecutorResultsMessage(ref),
			)

			es.ReleaseQueue()

			// TODO: if Current is not nil then we should request here for a delegation token
			// to continue execution of the current request

			es.Lock()
			if es.Current == nil {
				state.ExecutionState = nil
			}
			es.Unlock()
		}
	}

	for _, msg := range messages {
		_, err := lr.MessageBus.Send(ctx, msg, pulse, nil)
		if err != nil {
			return errors.New("error while sending caseBind data to new executor")
		}
	}

	return nil
}
