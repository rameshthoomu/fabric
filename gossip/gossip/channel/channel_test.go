/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package channel

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/comm"
	"github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/gossip/algo"
	"github.com/hyperledger/fabric/gossip/util"
	proto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type msgMutator func(message *proto.Envelope)

var conf = Config{
	ID: "test",
	PublishStateInfoInterval:    time.Millisecond * 100,
	MaxBlockCountToStore:        100,
	PullPeerNum:                 3,
	PullInterval:                time.Second,
	RequestStateInfoInterval:    time.Millisecond * 100,
	BlockExpirationInterval:     time.Second * 6,
	StateInfoExpirationInterval: time.Second * 6,
}

func init() {
	util.SetupTestLogging()
	shortenedWaitTime := time.Millisecond * 300
	algo.SetDigestWaitTime(shortenedWaitTime / 2)
	algo.SetRequestWaitTime(shortenedWaitTime)
	algo.SetResponseWaitTime(shortenedWaitTime)
	factory.InitFactories(nil)
}

var (
	// Organizations: {ORG1, ORG2}
	// Channel A: {ORG1}
	channelA                  = common.ChainID("A")
	orgInChannelA             = api.OrgIdentityType("ORG1")
	orgNotInChannelA          = api.OrgIdentityType("ORG2")
	pkiIDInOrg1               = common.PKIidType("pkiIDInOrg1")
	pkiIDnilOrg               = common.PKIidType("pkIDnilOrg")
	pkiIDInOrg1ButNotEligible = common.PKIidType("pkiIDInOrg1ButNotEligible")
	pkiIDinOrg2               = common.PKIidType("pkiIDinOrg2")
)

type joinChanMsg struct {
	getTS               func() time.Time
	members2AnchorPeers map[string][]api.AnchorPeer
}

// SequenceNumber returns the sequence number of the block
// this joinChanMsg was derived from.
// I use timestamps here just for the test.
func (jcm *joinChanMsg) SequenceNumber() uint64 {
	if jcm.getTS != nil {
		return uint64(jcm.getTS().UnixNano())
	}
	return uint64(time.Now().UnixNano())
}

// Members returns the organizations of the channel
func (jcm *joinChanMsg) Members() []api.OrgIdentityType {
	if jcm.members2AnchorPeers == nil {
		return []api.OrgIdentityType{orgInChannelA}
	}
	members := make([]api.OrgIdentityType, len(jcm.members2AnchorPeers))
	i := 0
	for org := range jcm.members2AnchorPeers {
		members[i] = api.OrgIdentityType(org)
		i++
	}
	return members
}

// AnchorPeersOf returns the anchor peers of the given organization
func (jcm *joinChanMsg) AnchorPeersOf(org api.OrgIdentityType) []api.AnchorPeer {
	if jcm.members2AnchorPeers == nil {
		return []api.AnchorPeer{}
	}
	return jcm.members2AnchorPeers[string(org)]
}

type cryptoService struct {
	mocked bool
	mock.Mock
}

func (cs *cryptoService) GetPKIidOfCert(peerIdentity api.PeerIdentityType) common.PKIidType {
	panic("Should not be called in this test")
}

func (cs *cryptoService) VerifyByChannel(channel common.ChainID, identity api.PeerIdentityType, _, _ []byte) error {
	if !cs.mocked {
		return nil
	}
	args := cs.Called(identity)
	return args.Get(0).(error)
}

func (cs *cryptoService) VerifyBlock(chainID common.ChainID, seqNum uint64, signedBlock []byte) error {
	args := cs.Called(signedBlock)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(error)
}

func (cs *cryptoService) Sign(msg []byte) ([]byte, error) {
	panic("Should not be called in this test")
}

func (cs *cryptoService) Verify(peerIdentity api.PeerIdentityType, signature, message []byte) error {
	panic("Should not be called in this test")
}

func (cs *cryptoService) ValidateIdentity(peerIdentity api.PeerIdentityType) error {
	panic("Should not be called in this test")
}

type receivedMsg struct {
	PKIID common.PKIidType
	msg   *proto.SignedGossipMessage
	mock.Mock
}

// GetSourceEnvelope Returns the Envelope the ReceivedMessage was
// constructed with
func (m *receivedMsg) GetSourceEnvelope() *proto.Envelope {
	return m.msg.Envelope
}

func (m *receivedMsg) GetGossipMessage() *proto.SignedGossipMessage {
	return m.msg
}

func (m *receivedMsg) Respond(msg *proto.GossipMessage) {
	m.Called(msg)
}

func (m *receivedMsg) GetConnectionInfo() *proto.ConnectionInfo {
	return &proto.ConnectionInfo{
		ID: m.PKIID,
	}
}

type gossipAdapterMock struct {
	mock.Mock
}

func (ga *gossipAdapterMock) GetConf() Config {
	args := ga.Called()
	return args.Get(0).(Config)
}

func (ga *gossipAdapterMock) Gossip(msg *proto.SignedGossipMessage) {
	ga.Called(msg)
}

func (ga *gossipAdapterMock) DeMultiplex(msg interface{}) {
	ga.Called(msg)
}

func (ga *gossipAdapterMock) GetMembership() []discovery.NetworkMember {
	args := ga.Called()
	members := args.Get(0).([]discovery.NetworkMember)
	return members
}

// Lookup returns a network member, or nil if not found
func (ga *gossipAdapterMock) Lookup(PKIID common.PKIidType) *discovery.NetworkMember {
	args := ga.Called(PKIID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*discovery.NetworkMember)
}

func (ga *gossipAdapterMock) Send(msg *proto.SignedGossipMessage, peers ...*comm.RemotePeer) {
	// Ensure we have configured Send prior
	foundSend := false
	for _, ec := range ga.ExpectedCalls {
		if ec.Method == "Send" {
			foundSend = true
		}

	}
	if !foundSend {
		return
	}
	ga.Called(msg, peers)
}

func (ga *gossipAdapterMock) ValidateStateInfoMessage(msg *proto.SignedGossipMessage) error {
	args := ga.Called(msg)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(error)
}

func (ga *gossipAdapterMock) GetOrgOfPeer(PKIIID common.PKIidType) api.OrgIdentityType {
	args := ga.Called(PKIIID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(api.OrgIdentityType)
}

func (ga *gossipAdapterMock) GetIdentityByPKIID(pkiID common.PKIidType) api.PeerIdentityType {
	return api.PeerIdentityType(pkiID)
}

func configureAdapter(adapter *gossipAdapterMock, members ...discovery.NetworkMember) {
	adapter.On("GetConf").Return(conf)
	adapter.On("GetMembership").Return(members)
	adapter.On("GetOrgOfPeer", pkiIDInOrg1).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDInOrg1ButNotEligible).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDinOrg2).Return(orgNotInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDnilOrg).Return(nil)
	adapter.On("GetOrgOfPeer", mock.Anything).Return(api.OrgIdentityType(nil))
}

func TestBadInput(t *testing.T) {
	cs := &cryptoService{}
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{}).(*gossipChannel)
	assert.False(t, gc.verifyMsg(nil))
	assert.False(t, gc.verifyMsg(&receivedMsg{msg: nil, PKIID: nil}))

	gc.UpdateStateInfo(createDataMsg(0, channelA).NoopSign())
	gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDnilOrg})
	gc.HandleMessage(&receivedMsg{msg: (&proto.GossipMessage{}).NoopSign()})
	gc.HandleMessage(&receivedMsg{msg: createDataUpdateMsg(0), PKIID: pkiIDnilOrg})
}

func TestChannelPeriodicalPublishStateInfo(t *testing.T) {
	t.Parallel()
	ledgerHeight := 5
	receivedMsg := int32(0)
	stateInfoReceptionChan := make(chan *proto.SignedGossipMessage, 1)

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)

	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("Gossip", mock.Anything).Run(func(arg mock.Arguments) {
		if atomic.LoadInt32(&receivedMsg) == int32(1) {
			return
		}

		atomic.StoreInt32(&receivedMsg, int32(1))
		msg := arg.Get(0).(*proto.SignedGossipMessage)
		stateInfoReceptionChan <- msg
	})

	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	stateInfoMsg := createStateInfoMsg(ledgerHeight, pkiIDInOrg1, channelA)
	gc.UpdateStateInfo(stateInfoMsg)

	var msg *proto.SignedGossipMessage
	select {
	case <-time.After(time.Second * 5):
		t.Fatal("Haven't sent stateInfo on time")
	case m := <-stateInfoReceptionChan:
		msg = m
	}

	md := msg.GetStateInfo().Metadata
	height, err := strconv.ParseInt(string(md), 10, 64)
	assert.NoError(t, err, "ReceivedMetadata is invalid")
	assert.Equal(t, ledgerHeight, int(height), "Received different ledger height than expected")

	// We will not update StateInfo in store, so store will become empty
	time.Sleep(conf.StateInfoExpirationInterval + time.Second)
	//Store is empty
	assert.Equal(t, 0, gc.(*gossipChannel).stateInfoMsgStore.MessageStore.Size(), "StateInfo MessageStore should be empty")
	assert.Equal(t, 0, gc.(*gossipChannel).stateInfoMsgStore.MembershipStore.Size(), "StateInfo MembershipStore should be empty")

	gc.Stop()
}

func TestChannelMsgStoreEviction(t *testing.T) {
	t.Parallel()
	// Scenario: Create 4 phases in which the pull mediator of the channel would receive blocks
	// via pull.
	// The total amount of blocks should be restricted by the capacity of the message store.
	// After the pull phases end, we ensure that only the latest blocks are preserved in the pull
	// mediator, and the old blocks were evicted.
	// We test this by sending a hello message to the pull mediator and inspecting the digest message
	// returned as a response.

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	adapter.On("Gossip", mock.Anything)
	adapter.On("DeMultiplex", mock.Anything).Run(func(arg mock.Arguments) {
	})

	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	defer gc.Stop()
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(100, pkiIDInOrg1, channelA)})

	var wg sync.WaitGroup

	msgsPerPhase := uint64(50)
	lastPullPhase := make(chan uint64, msgsPerPhase)
	totalPhases := uint64(4)
	phaseNum := uint64(0)
	wg.Add(int(totalPhases))

	adapter.On("Send", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		msg := args.Get(0).(*proto.SignedGossipMessage)
		// Ignore all other messages sent like StateInfo messages
		if !msg.IsPullMsg() {
			return
		}
		// Stop the pull when we reach the final phase
		if atomic.LoadUint64(&phaseNum) == totalPhases && msg.IsHelloMsg() {
			return
		}

		start := atomic.LoadUint64(&phaseNum) * msgsPerPhase
		end := start + msgsPerPhase
		if msg.IsHelloMsg() {
			// Advance phase
			atomic.AddUint64(&phaseNum, uint64(1))
		}

		// Create and execute the current phase of pull
		currSeq := sequence(start, end)
		pullPhase := simulatePullPhase(gc, t, &wg, func(envelope *proto.Envelope) {}, currSeq...)
		pullPhase(args)

		// If we finished the last phase, save the sequence to be used later for inspection
		if msg.IsDataReq() && atomic.LoadUint64(&phaseNum) == totalPhases {
			for _, seq := range currSeq {
				lastPullPhase <- seq
			}
			close(lastPullPhase)
		}
	})
	// Wait for all pull phases to end
	wg.Wait()

	msgSentFromPullMediator := make(chan *proto.GossipMessage, 1)

	helloMsg := createHelloMsg(pkiIDInOrg1)
	helloMsg.On("Respond", mock.Anything).Run(func(arg mock.Arguments) {
		msg := arg.Get(0).(*proto.GossipMessage)
		if !msg.IsDigestMsg() {
			return
		}
		msgSentFromPullMediator <- msg
	})
	gc.HandleMessage(helloMsg)
	select {
	case msg := <-msgSentFromPullMediator:
		// This is just to check that we responded with a digest on time.
		// Put message back into the channel for further inspection
		msgSentFromPullMediator <- msg
	case <-time.After(time.Second * 5):
		t.Fatal("Didn't reply with a digest on time")
	}
	// Only 1 digest sent
	assert.Len(t, msgSentFromPullMediator, 1)
	msg := <-msgSentFromPullMediator
	// It's a digest and not anything else, like an update
	assert.True(t, msg.IsDigestMsg())
	assert.Len(t, msg.GetDataDig().Digests, adapter.GetConf().MaxBlockCountToStore+1)
	// Check that the last sequences are kept.
	// Since we checked the length, it proves that the old blocks were discarded, since we had much more
	// total blocks overall than our capacity
	for seq := range lastPullPhase {
		assert.Contains(t, msg.GetDataDig().Digests, fmt.Sprintf("%d", seq))
	}
}

func TestChannelPull(t *testing.T) {
	t.Parallel()
	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	receivedBlocksChan := make(chan *proto.SignedGossipMessage)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	adapter.On("Gossip", mock.Anything)
	adapter.On("DeMultiplex", mock.Anything).Run(func(arg mock.Arguments) {
		msg := arg.Get(0).(*proto.SignedGossipMessage)
		if !msg.IsDataMsg() {
			return
		}
		// The peer is supposed to de-multiplex 2 ledger blocks
		assert.True(t, msg.IsDataMsg())
		receivedBlocksChan <- msg
	})
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	go gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(100, pkiIDInOrg1, channelA)})

	var wg sync.WaitGroup
	pullPhase := simulatePullPhase(gc, t, &wg, func(envelope *proto.Envelope) {}, 10, 11)
	adapter.On("Send", mock.Anything, mock.Anything).Run(pullPhase)

	wg.Wait()
	for expectedSeq := 10; expectedSeq < 11; expectedSeq++ {
		select {
		case <-time.After(time.Second * 5):
			t.Fatal("Haven't received blocks on time")
		case msg := <-receivedBlocksChan:
			assert.Equal(t, uint64(expectedSeq), msg.GetDataMsg().Payload.SeqNum)
		}
	}
}

func TestChannelPeerNotInChannel(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	gossipMessagesSentFromChannel := make(chan *proto.GossipMessage, 1)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})

	// First thing, we test that blocks can only be received from peers that are in an org that's in the channel
	// Empty PKI-ID, should drop the block
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(5, channelA)})
	assert.Equal(t, 0, gc.(*gossipChannel).blockMsgStore.Size())

	// Known PKI-ID but not in channel, should drop the block
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(5, channelA), PKIID: pkiIDinOrg2})
	assert.Equal(t, 0, gc.(*gossipChannel).blockMsgStore.Size())
	// Known PKI-ID, and in channel, should add the block
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(5, channelA), PKIID: pkiIDInOrg1})
	assert.Equal(t, 1, gc.(*gossipChannel).blockMsgStore.Size())

	// Next, we make sure that the channel doesn't respond to pull messages (hello or requests) from peers that're not in the channel
	messageRelayer := func(arg mock.Arguments) {
		msg := arg.Get(0).(*proto.GossipMessage)
		gossipMessagesSentFromChannel <- msg
	}
	// First, ensure it does that for pull messages from peers that are in the channel
	// Let the peer first publish it is in the channel
	gc.HandleMessage(&receivedMsg{msg: createStateInfoMsg(10, pkiIDInOrg1, channelA), PKIID: pkiIDInOrg1})
	helloMsg := createHelloMsg(pkiIDInOrg1)
	helloMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(helloMsg)
	select {
	case <-gossipMessagesSentFromChannel:
	case <-time.After(time.Second * 5):
		t.Fatal("Didn't reply with a digest on time")
	}
	// And now for peers that are not in the channel (should not send back a message)
	helloMsg = createHelloMsg(pkiIDinOrg2)
	helloMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(helloMsg)
	select {
	case <-gossipMessagesSentFromChannel:
		t.Fatal("Responded with digest, but shouldn't have since peer is in ORG2 and its not in the channel")
	case <-time.After(time.Second * 1):
	}

	// Now for a more advanced scenario- the peer claims to be in the right org, and also claims to be in the channel
	// but the MSP declares it is not eligible for the channel
	// pkiIDInOrg1ButNotEligible
	gc.HandleMessage(&receivedMsg{msg: createStateInfoMsg(10, pkiIDInOrg1ButNotEligible, channelA), PKIID: pkiIDInOrg1ButNotEligible})
	cs.On("VerifyByChannel", mock.Anything).Return(errors.New("Not eligible"))
	cs.mocked = true
	helloMsg = createHelloMsg(pkiIDInOrg1ButNotEligible)
	helloMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(helloMsg)
	select {
	case <-gossipMessagesSentFromChannel:
		t.Fatal("Responded with digest, but shouldn't have since peer is not eligible for the channel")
	case <-time.After(time.Second * 1):
	}

	cs.Mock = mock.Mock{}

	// Ensure we respond to a valid StateInfoRequest
	req := gc.(*gossipChannel).createStateInfoRequest()
	validReceivedMsg := &receivedMsg{
		msg:   req,
		PKIID: pkiIDInOrg1,
	}
	validReceivedMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(validReceivedMsg)
	select {
	case <-gossipMessagesSentFromChannel:
	case <-time.After(time.Second * 5):
		t.Fatal("Didn't reply with a digest on time")
	}

	// Ensure we don't respond to a StateInfoRequest from a peer in the wrong org
	invalidReceivedMsg := &receivedMsg{
		msg:   req,
		PKIID: pkiIDinOrg2,
	}
	invalidReceivedMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(invalidReceivedMsg)
	select {
	case <-gossipMessagesSentFromChannel:
		t.Fatal("Responded with digest, but shouldn't have since peer is in ORG2 and its not in the channel")
	case <-time.After(time.Second * 1):
	}

	// Ensure we don't respond to a StateInfoRequest in the wrong channel from a peer in the right org
	req2 := gc.(*gossipChannel).createStateInfoRequest()
	req2.GetStateInfoPullReq().ChannelMAC = GenerateMAC(pkiIDInOrg1, common.ChainID("B"))
	invalidReceivedMsg2 := &receivedMsg{
		msg:   req2,
		PKIID: pkiIDInOrg1,
	}
	invalidReceivedMsg2.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(invalidReceivedMsg2)
	select {
	case <-gossipMessagesSentFromChannel:
		t.Fatal("Responded with stateInfo request, but shouldn't have since it has the wrong MAC")
	case <-time.After(time.Second * 1):
	}
}

func TestChannelIsInChannel(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)

	assert.False(t, gc.IsOrgInChannel(nil))
	assert.True(t, gc.IsOrgInChannel(orgInChannelA))
	assert.False(t, gc.IsOrgInChannel(orgNotInChannelA))
	assert.True(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDInOrg1}))
	assert.False(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDinOrg2}))
}

func TestChannelIsSubscribed(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	gc.HandleMessage(&receivedMsg{msg: createStateInfoMsg(10, pkiIDInOrg1, channelA), PKIID: pkiIDInOrg1})
	assert.True(t, gc.EligibleForChannel(discovery.NetworkMember{PKIid: pkiIDInOrg1}))
}

func TestChannelAddToMessageStore(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	demuxedMsgs := make(chan *proto.SignedGossipMessage, 1)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything).Run(func(arg mock.Arguments) {
		demuxedMsgs <- arg.Get(0).(*proto.SignedGossipMessage)
	})

	// Check that adding a message of a bad type doesn't crash the program
	gc.AddToMsgStore(createHelloMsg(pkiIDInOrg1).GetGossipMessage())

	// We make sure that if we get a new message it is de-multiplexed,
	// but if we put such a message in the message store, it isn't demultiplexed when we
	// receive that message again
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(11, channelA), PKIID: pkiIDInOrg1})
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't detected a demultiplexing within a time period")
	case <-demuxedMsgs:
	}
	gc.AddToMsgStore(dataMsgOfChannel(12, channelA))
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(12, channelA), PKIID: pkiIDInOrg1})
	select {
	case <-time.After(time.Second):
	case <-demuxedMsgs:
		t.Fatal("Demultiplexing detected, even though it wasn't supposed to happen")
	}

	gc.AddToMsgStore(createStateInfoMsg(10, pkiIDInOrg1, channelA))
	helloMsg := createHelloMsg(pkiIDInOrg1)
	respondedChan := make(chan struct{}, 1)
	helloMsg.On("Respond", mock.Anything).Run(func(arg mock.Arguments) {
		respondedChan <- struct{}{}
	})
	gc.HandleMessage(helloMsg)
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't responded to hello message within a time period")
	case <-respondedChan:
	}

	gc.HandleMessage(&receivedMsg{msg: createStateInfoMsg(10, pkiIDInOrg1, channelA), PKIID: pkiIDInOrg1})
	assert.True(t, gc.EligibleForChannel(discovery.NetworkMember{PKIid: pkiIDInOrg1}))
}

func TestChannelAddToMessageStoreExpire(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	demuxedMsgs := make(chan *proto.SignedGossipMessage, 1)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything).Run(func(arg mock.Arguments) {
		demuxedMsgs <- arg.Get(0).(*proto.SignedGossipMessage)
	})

	respondedChan := make(chan *proto.GossipMessage, 1)
	messageRelayer := func(arg mock.Arguments) {
		msg := arg.Get(0).(*proto.GossipMessage)
		respondedChan <- msg
	}

	// We make sure that if we get a new message it is de-multiplexed,
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(11, channelA), PKIID: pkiIDInOrg1})
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't detected a demultiplexing within a time period")
	case <-demuxedMsgs:
	}

	// Lets check digests and state info store
	stateInfoMsg := createStateInfoMsg(10, pkiIDInOrg1, channelA)
	gc.AddToMsgStore(stateInfoMsg)
	helloMsg := createHelloMsg(pkiIDInOrg1)
	helloMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(helloMsg)
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't responded to hello message within a time period")
	case msg := <-respondedChan:
		if msg.IsDigestMsg() {
			assert.Equal(t, 1, len(msg.GetDataDig().Digests), "Number of digests returned by channel blockPuller incorrect")
		} else {
			t.Fatal("Not correct pull msg type in responce - expect digest")
		}
	}

	time.Sleep(gc.(*gossipChannel).GetConf().BlockExpirationInterval + time.Second)

	// message expired in store, but still isn't demultiplexed when we
	// receive that message again
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(11, channelA), PKIID: pkiIDInOrg1})
	select {
	case <-time.After(time.Second):
	case <-demuxedMsgs:
		t.Fatal("Demultiplexing detected, even though it wasn't supposed to happen")
	}

	// Lets check digests and state info store - state info expired, its add will do nothing and digest should not be sent
	gc.AddToMsgStore(stateInfoMsg)
	gc.HandleMessage(helloMsg)
	select {
	case <-time.After(time.Second):
	case <-respondedChan:
		t.Fatal("No digest should be sent")
	}

	time.Sleep(gc.(*gossipChannel).GetConf().BlockExpirationInterval + time.Second)
	// message removed from store, so it will be demultiplexed when we
	// receive that message again
	gc.HandleMessage(&receivedMsg{msg: dataMsgOfChannel(11, channelA), PKIID: pkiIDInOrg1})
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't detected a demultiplexing within a time period")
	case <-demuxedMsgs:
	}

	// Lets check digests and state info store - state info removed as well, so it will be added back and digest will be created
	gc.AddToMsgStore(stateInfoMsg)
	gc.HandleMessage(helloMsg)
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't responded to hello message within a time period")
	case msg := <-respondedChan:
		if msg.IsDigestMsg() {
			assert.Equal(t, 1, len(msg.GetDataDig().Digests), "Number of digests returned by channel blockPuller incorrect")
		} else {
			t.Fatal("Not correct pull msg type in responce - expect digest")
		}
	}

	gc.Stop()
}

func TestChannelBadBlocks(t *testing.T) {
	t.Parallel()
	receivedMessages := make(chan *proto.SignedGossipMessage, 1)
	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	adapter.On("Gossip", mock.Anything)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})

	adapter.On("DeMultiplex", mock.Anything).Run(func(args mock.Arguments) {
		receivedMessages <- args.Get(0).(*proto.SignedGossipMessage)
	})

	// Send a valid block
	gc.HandleMessage(&receivedMsg{msg: createDataMsg(1, channelA), PKIID: pkiIDInOrg1})
	assert.Len(t, receivedMessages, 1)
	<-receivedMessages // drain

	// Send a block with wrong channel
	gc.HandleMessage(&receivedMsg{msg: createDataMsg(2, common.ChainID("B")), PKIID: pkiIDInOrg1})
	assert.Len(t, receivedMessages, 0)

	// Send a block with empty payload
	dataMsg := createDataMsg(3, channelA)
	dataMsg.GetDataMsg().Payload = nil
	gc.HandleMessage(&receivedMsg{msg: dataMsg, PKIID: pkiIDInOrg1})
	assert.Len(t, receivedMessages, 0)

	// Send a block with a bad signature
	cs.Mock = mock.Mock{}
	cs.On("VerifyBlock", mock.Anything).Return(errors.New("Bad signature"))
	gc.HandleMessage(&receivedMsg{msg: createDataMsg(4, channelA), PKIID: pkiIDInOrg1})
	assert.Len(t, receivedMessages, 0)
}

func TestChannelPulledBadBlocks(t *testing.T) {
	t.Parallel()

	// Test a pull with a block of a bad channel
	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	adapter.On("DeMultiplex", mock.Anything)
	adapter.On("Gossip", mock.Anything)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDInOrg1, channelA)})

	var wg sync.WaitGroup
	wg.Add(1)

	changeChan := func(env *proto.Envelope) {
		sMsg, _ := env.ToGossipMessage()
		sMsg.Channel = []byte("B")
		env.Payload = sMsg.NoopSign().Payload
	}

	pullPhase1 := simulatePullPhase(gc, t, &wg, changeChan, 10, 11)
	adapter.On("Send", mock.Anything, mock.Anything).Run(pullPhase1)
	adapter.On("DeMultiplex", mock.Anything)
	wg.Wait()
	gc.Stop()
	assert.Equal(t, 0, gc.(*gossipChannel).blockMsgStore.Size())

	// Test a pull with a badly signed block
	cs = &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(errors.New("Bad block"))
	adapter = new(gossipAdapterMock)
	adapter.On("Gossip", mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	gc = NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDInOrg1, channelA)})

	var wg2 sync.WaitGroup
	wg2.Add(1)
	noop := func(env *proto.Envelope) {

	}
	pullPhase2 := simulatePullPhase(gc, t, &wg2, noop, 10, 11)
	adapter.On("Send", mock.Anything, mock.Anything).Run(pullPhase2)
	wg2.Wait()
	assert.Equal(t, 0, gc.(*gossipChannel).blockMsgStore.Size())

	// Test a pull with an empty block
	cs = &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter = new(gossipAdapterMock)
	adapter.On("Gossip", mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	gc = NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDInOrg1, channelA)})

	var wg3 sync.WaitGroup
	wg3.Add(1)
	emptyBlock := func(env *proto.Envelope) {
		sMsg, _ := env.ToGossipMessage()
		sMsg.GossipMessage.GetDataMsg().Payload = nil
		env.Payload = sMsg.NoopSign().Payload
	}
	pullPhase3 := simulatePullPhase(gc, t, &wg3, emptyBlock, 10, 11)
	adapter.On("Send", mock.Anything, mock.Anything).Run(pullPhase3)
	wg3.Wait()
	assert.Equal(t, 0, gc.(*gossipChannel).blockMsgStore.Size())

	// Test a pull with a non-block message
	cs = &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)

	adapter = new(gossipAdapterMock)
	adapter.On("Gossip", mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	gc = NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDInOrg1, channelA)})

	var wg4 sync.WaitGroup
	wg4.Add(1)
	nonBlockMsg := func(env *proto.Envelope) {
		sMsg, _ := env.ToGossipMessage()
		sMsg.Content = createHelloMsg(pkiIDInOrg1).GetGossipMessage().Content
		env.Payload = sMsg.NoopSign().Payload
	}
	pullPhase4 := simulatePullPhase(gc, t, &wg4, nonBlockMsg, 10, 11)
	adapter.On("Send", mock.Anything, mock.Anything).Run(pullPhase4)
	wg4.Wait()
	assert.Equal(t, 0, gc.(*gossipChannel).blockMsgStore.Size())
}

func TestChannelStateInfoSnapshot(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	adapter.On("Lookup", mock.Anything).Return(&discovery.NetworkMember{Endpoint: "localhost:5000"})
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	adapter.On("Gossip", mock.Anything)
	sentMessages := make(chan *proto.GossipMessage, 10)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("ValidateStateInfoMessage", mock.Anything).Return(nil)

	// Ensure we ignore stateInfo snapshots from peers not in the channel
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: stateInfoSnapshotForChannel(common.ChainID("B"), createStateInfoMsg(4, pkiIDInOrg1, channelA))})
	assert.Empty(t, gc.GetPeers())
	// Ensure we ignore invalid stateInfo snapshots
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, pkiIDInOrg1, common.ChainID("B")))})
	assert.Empty(t, gc.GetPeers())

	// Ensure we ignore stateInfo messages from peers not in the channel
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, pkiIDinOrg2, channelA))})
	assert.Empty(t, gc.GetPeers())

	// Ensure we ignore stateInfo snapshots from peers not in the org
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDinOrg2, msg: stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, pkiIDInOrg1, channelA))})
	assert.Empty(t, gc.GetPeers())

	// Ensure we ignore stateInfo snapshots with StateInfo messages with wrong MACs
	sim := createStateInfoMsg(4, pkiIDInOrg1, channelA)
	sim.GetStateInfo().ChannelMAC = append(sim.GetStateInfo().ChannelMAC, 1)
	sim = sim.NoopSign()
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: stateInfoSnapshotForChannel(channelA, sim)})
	assert.Empty(t, gc.GetPeers())

	// Ensure we ignore stateInfo snapshots with correct StateInfo messages, BUT with wrong MACs
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, pkiIDInOrg1, channelA))})

	// Ensure we process stateInfo snapshots that are OK
	stateInfoMsg := &receivedMsg{PKIID: pkiIDInOrg1, msg: stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, pkiIDInOrg1, channelA))}
	gc.HandleMessage(stateInfoMsg)
	assert.NotEmpty(t, gc.GetPeers())
	assert.Equal(t, "4", string(gc.GetPeers()[0].Metadata))

	// Check we don't respond to stateInfoSnapshot requests with wrong MAC
	snapshotReq := &receivedMsg{
		PKIID: pkiIDInOrg1,
		msg: (&proto.GossipMessage{
			Tag: proto.GossipMessage_CHAN_OR_ORG,
			Content: &proto.GossipMessage_StateInfoPullReq{
				StateInfoPullReq: &proto.StateInfoPullRequest{
					ChannelMAC: append(GenerateMAC(pkiIDInOrg1, channelA), 1),
				},
			},
		}).NoopSign(),
	}
	snapshotReq.On("Respond", mock.Anything).Run(func(args mock.Arguments) {
		sentMessages <- args.Get(0).(*proto.GossipMessage)
	})

	go gc.HandleMessage(snapshotReq)
	select {
	case <-time.After(time.Second):
	case <-sentMessages:
		assert.Fail(t, "Shouldn't have responded to this StateInfoSnapshot request because of bad MAC")
	}

	// Ensure we respond to stateInfoSnapshot requests with valid MAC
	snapshotReq = &receivedMsg{
		PKIID: pkiIDInOrg1,
		msg: (&proto.GossipMessage{
			Tag: proto.GossipMessage_CHAN_OR_ORG,
			Content: &proto.GossipMessage_StateInfoPullReq{
				StateInfoPullReq: &proto.StateInfoPullRequest{
					ChannelMAC: GenerateMAC(pkiIDInOrg1, channelA),
				},
			},
		}).NoopSign(),
	}
	snapshotReq.On("Respond", mock.Anything).Run(func(args mock.Arguments) {
		sentMessages <- args.Get(0).(*proto.GossipMessage)
	})

	go gc.HandleMessage(snapshotReq)
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't received a state info snapshot on time")
	case msg := <-sentMessages:
		elements := msg.GetStateSnapshot().Elements
		assert.Len(t, elements, 1)
		sMsg, err := elements[0].ToGossipMessage()
		assert.NoError(t, err)
		assert.Equal(t, []byte("4"), sMsg.GetStateInfo().Metadata)
	}

	// Ensure we don't crash if we got an invalid state info message
	invalidStateInfoSnapshot := stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, pkiIDInOrg1, channelA))
	invalidStateInfoSnapshot.GetStateSnapshot().Elements = []*proto.Envelope{createHelloMsg(pkiIDInOrg1).GetSourceEnvelope()}
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: invalidStateInfoSnapshot})

	// Ensure we don't crash if we got a stateInfoMessage from a peer that its org isn't known
	invalidStateInfoSnapshot = stateInfoSnapshotForChannel(channelA, createStateInfoMsg(4, common.PKIidType("unknown"), channelA))
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: invalidStateInfoSnapshot})
	// Lets expire msg in store
	time.Sleep(gc.(*gossipChannel).GetConf().StateInfoExpirationInterval + time.Second)

	// Lets check is state info store can't add expired msg but appear as empty to outside world
	gc.HandleMessage(stateInfoMsg)
	assert.Empty(t, gc.GetPeers())
	// Lets see if snapshot now empty, after message in store expired
	go gc.HandleMessage(snapshotReq)
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't received a state info snapshot on time")
	case msg := <-sentMessages:
		elements := msg.GetStateSnapshot().Elements
		assert.Len(t, elements, 0, "StateInfo snapshot should contain zero messages")
	}

	// Lets make sure msg removed from store
	time.Sleep(gc.(*gossipChannel).GetConf().StateInfoExpirationInterval + time.Second)

	// Lets check is state info store add just expired msg
	gc.HandleMessage(stateInfoMsg)
	assert.NotEmpty(t, gc.GetPeers())
	// Lets see if snapshot is not empty now, after message was added back to store
	go gc.HandleMessage(snapshotReq)
	select {
	case <-time.After(time.Second):
		t.Fatal("Haven't received a state info snapshot on time")
	case msg := <-sentMessages:
		elements := msg.GetStateSnapshot().Elements
		assert.Len(t, elements, 1)
		sMsg, err := elements[0].ToGossipMessage()
		assert.NoError(t, err)
		assert.Equal(t, []byte("4"), sMsg.GetStateInfo().Metadata)
	}
}

func TestInterOrgExternalEndpointDisclosure(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	adapter := new(gossipAdapterMock)
	pkiID1 := common.PKIidType("withExternalEndpoint")
	pkiID2 := common.PKIidType("noExternalEndpoint")
	pkiID3 := common.PKIidType("pkiIDinOrg2")
	adapter.On("Lookup", pkiID1).Return(&discovery.NetworkMember{Endpoint: "localhost:5000"})
	adapter.On("Lookup", pkiID2).Return(&discovery.NetworkMember{})
	adapter.On("GetOrgOfPeer", pkiID1).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiID2).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiID3).Return(api.OrgIdentityType("ORG2"))
	adapter.On("Gossip", mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	configureAdapter(adapter)
	jcm := &joinChanMsg{
		members2AnchorPeers: map[string][]api.AnchorPeer{
			string(orgInChannelA): {},
			"ORG2":                {},
		},
	}
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, jcm)
	gc.HandleMessage(&receivedMsg{PKIID: pkiID1, msg: createStateInfoMsg(0, pkiID1, channelA)})
	gc.HandleMessage(&receivedMsg{PKIID: pkiID2, msg: createStateInfoMsg(0, pkiID2, channelA)})
	gc.HandleMessage(&receivedMsg{PKIID: pkiID2, msg: createStateInfoMsg(0, pkiID3, channelA)})

	sentMessages := make(chan *proto.GossipMessage, 10)

	// Check that we only return StateInfo messages of peers with external endpoints
	// to peers of other orgs
	snapshotReq := &receivedMsg{
		PKIID: pkiID3,
		msg: (&proto.GossipMessage{
			Tag: proto.GossipMessage_CHAN_OR_ORG,
			Content: &proto.GossipMessage_StateInfoPullReq{
				StateInfoPullReq: &proto.StateInfoPullRequest{
					ChannelMAC: GenerateMAC(pkiID3, channelA),
				},
			},
		}).NoopSign(),
	}
	snapshotReq.On("Respond", mock.Anything).Run(func(args mock.Arguments) {
		sentMessages <- args.Get(0).(*proto.GossipMessage)
	})

	go gc.HandleMessage(snapshotReq)
	select {
	case <-time.After(time.Second):
		assert.Fail(t, "Should have responded to this StateInfoSnapshot, but didn't")
	case msg := <-sentMessages:
		elements := msg.GetStateSnapshot().Elements
		assert.Len(t, elements, 2)
		m1, _ := elements[0].ToGossipMessage()
		m2, _ := elements[1].ToGossipMessage()
		pkiIDs := [][]byte{m1.GetStateInfo().PkiId, m2.GetStateInfo().PkiId}
		assert.Contains(t, pkiIDs, []byte(pkiID1))
		assert.Contains(t, pkiIDs, []byte(pkiID3))
	}

	// Check that we return all StateInfo messages to peers in our organization, regardless
	// if the peers from foreign organizations have external endpoints or not
	snapshotReq = &receivedMsg{
		PKIID: pkiID2,
		msg: (&proto.GossipMessage{
			Tag: proto.GossipMessage_CHAN_OR_ORG,
			Content: &proto.GossipMessage_StateInfoPullReq{
				StateInfoPullReq: &proto.StateInfoPullRequest{
					ChannelMAC: GenerateMAC(pkiID2, channelA),
				},
			},
		}).NoopSign(),
	}
	snapshotReq.On("Respond", mock.Anything).Run(func(args mock.Arguments) {
		sentMessages <- args.Get(0).(*proto.GossipMessage)
	})

	go gc.HandleMessage(snapshotReq)
	select {
	case <-time.After(time.Second):
		assert.Fail(t, "Should have responded to this StateInfoSnapshot, but didn't")
	case msg := <-sentMessages:
		elements := msg.GetStateSnapshot().Elements
		assert.Len(t, elements, 3)
		m1, _ := elements[0].ToGossipMessage()
		m2, _ := elements[1].ToGossipMessage()
		m3, _ := elements[2].ToGossipMessage()
		pkiIDs := [][]byte{m1.GetStateInfo().PkiId, m2.GetStateInfo().PkiId, m3.GetStateInfo().PkiId}
		assert.Contains(t, pkiIDs, []byte(pkiID1))
		assert.Contains(t, pkiIDs, []byte(pkiID2))
		assert.Contains(t, pkiIDs, []byte(pkiID3))
	}
}

func TestChannelStop(t *testing.T) {
	t.Parallel()

	cs := &cryptoService{}
	cs.On("VerifyBlock", mock.Anything).Return(nil)
	adapter := new(gossipAdapterMock)
	var sendCount int32
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})
	adapter.On("Send", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		atomic.AddInt32(&sendCount, int32(1))
	})
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	time.Sleep(time.Second)
	gc.Stop()
	oldCount := atomic.LoadInt32(&sendCount)
	t1 := time.Now()
	for {
		if time.Since(t1).Nanoseconds() > (time.Second * 15).Nanoseconds() {
			t.Fatal("Stop failed")
		}
		time.Sleep(time.Second)
		newCount := atomic.LoadInt32(&sendCount)
		if newCount == oldCount {
			break
		}
		oldCount = newCount
	}
}

func TestChannelReconfigureChannel(t *testing.T) {
	t.Parallel()

	// Scenario: We test the following things:
	// Updating a channel with an outdated JoinChannel message doesn't work
	// Removing an organization from a channel is indeed reflected in that
	// the GossipChannel doesn't consider peers from that organization as
	// peers in the channel, and refuses to have any channel-related contact
	// with peers of that channel

	cs := &cryptoService{}
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})

	adapter.On("GetConf").Return(conf)
	adapter.On("GetMembership").Return([]discovery.NetworkMember{})
	adapter.On("OrgByPeerIdentity", api.PeerIdentityType(orgInChannelA)).Return(orgInChannelA)
	adapter.On("OrgByPeerIdentity", api.PeerIdentityType(orgNotInChannelA)).Return(orgNotInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDInOrg1).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDinOrg2).Return(orgNotInChannelA)

	outdatedJoinChanMsg := &joinChanMsg{
		getTS: func() time.Time {
			return time.Now()
		},
		members2AnchorPeers: map[string][]api.AnchorPeer{
			string(orgNotInChannelA): {},
		},
	}

	newJoinChanMsg := &joinChanMsg{
		getTS: func() time.Time {
			return time.Now().Add(time.Millisecond * 100)
		},
		members2AnchorPeers: map[string][]api.AnchorPeer{
			string(orgInChannelA): {},
		},
	}

	updatedJoinChanMsg := &joinChanMsg{
		getTS: func() time.Time {
			return time.Now().Add(time.Millisecond * 200)
		},
		members2AnchorPeers: map[string][]api.AnchorPeer{
			string(orgNotInChannelA): {},
		},
	}

	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, api.JoinChannelMessage(newJoinChanMsg))

	// Just call it again, to make sure stuff don't crash
	gc.ConfigureChannel(api.JoinChannelMessage(newJoinChanMsg))

	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)

	assert.True(t, gc.IsOrgInChannel(orgInChannelA))
	assert.False(t, gc.IsOrgInChannel(orgNotInChannelA))
	assert.True(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDInOrg1}))
	assert.False(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDinOrg2}))

	gc.ConfigureChannel(outdatedJoinChanMsg)
	assert.True(t, gc.IsOrgInChannel(orgInChannelA))
	assert.False(t, gc.IsOrgInChannel(orgNotInChannelA))
	assert.True(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDInOrg1}))
	assert.False(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDinOrg2}))

	gc.ConfigureChannel(updatedJoinChanMsg)
	gc.ConfigureChannel(updatedJoinChanMsg)
	assert.False(t, gc.IsOrgInChannel(orgInChannelA))
	assert.True(t, gc.IsOrgInChannel(orgNotInChannelA))
	assert.False(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDInOrg1}))
	assert.True(t, gc.IsMemberInChan(discovery.NetworkMember{PKIid: pkiIDinOrg2}))

	// Ensure we don't respond to a StateInfoRequest from a peer in the wrong org
	invalidReceivedMsg := &receivedMsg{
		msg:   gc.(*gossipChannel).createStateInfoRequest(),
		PKIID: pkiIDInOrg1,
	}
	gossipMessagesSentFromChannel := make(chan *proto.GossipMessage, 1)
	messageRelayer := func(arg mock.Arguments) {
		msg := arg.Get(0).(*proto.GossipMessage)
		gossipMessagesSentFromChannel <- msg
	}
	invalidReceivedMsg.On("Respond", mock.Anything).Run(messageRelayer)
	gc.HandleMessage(invalidReceivedMsg)
	select {
	case <-gossipMessagesSentFromChannel:
		t.Fatal("Responded with digest, but shouldn't have since peer is in ORG2 and its not in the channel")
	case <-time.After(time.Second * 1):
	}
}

func TestChannelNoAnchorPeers(t *testing.T) {
	t.Parallel()

	// Scenario: We got a join channel message with no anchor peers
	// In this case, we should be in the channel

	cs := &cryptoService{}
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter, discovery.NetworkMember{PKIid: pkiIDInOrg1})

	adapter.On("GetConf").Return(conf)
	adapter.On("GetMembership").Return([]discovery.NetworkMember{})
	adapter.On("OrgByPeerIdentity", api.PeerIdentityType(orgInChannelA)).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDInOrg1).Return(orgInChannelA)
	adapter.On("GetOrgOfPeer", pkiIDinOrg2).Return(orgNotInChannelA)

	jcm := &joinChanMsg{
		members2AnchorPeers: map[string][]api.AnchorPeer{
			string(orgInChannelA): {},
		},
	}

	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, api.JoinChannelMessage(jcm))
	assert.True(t, gc.IsOrgInChannel(orgInChannelA))
}

func TestChannelGetPeers(t *testing.T) {
	t.Parallel()

	// Scenario: We have a peer in an org, and the peer is notified that several peers
	// exist, and some of them:
	// (1) Join its channel, and are eligible for receiving blocks.
	// (2) Join its channel, but are not eligible for receiving blocks (MSP doesn't allow this).
	// (3) Say they join its channel, but are actually from an org that is not in the channel.
	// The GetPeers query should only return peers that belong to the first group.
	cs := &cryptoService{}
	adapter := new(gossipAdapterMock)
	adapter.On("Gossip", mock.Anything)
	adapter.On("Send", mock.Anything, mock.Anything)
	adapter.On("DeMultiplex", mock.Anything)
	members := []discovery.NetworkMember{
		{PKIid: pkiIDInOrg1},
		{PKIid: pkiIDInOrg1ButNotEligible},
		{PKIid: pkiIDinOrg2},
	}
	configureAdapter(adapter, members...)
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDInOrg1, channelA)})
	gc.HandleMessage(&receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDinOrg2, channelA)})
	assert.Len(t, gc.GetPeers(), 1)
	assert.Equal(t, pkiIDInOrg1, gc.GetPeers()[0].PKIid)

	gc.HandleMessage(&receivedMsg{msg: createStateInfoMsg(10, pkiIDInOrg1ButNotEligible, channelA), PKIID: pkiIDInOrg1ButNotEligible})
	cs.On("VerifyByChannel", mock.Anything).Return(errors.New("Not eligible"))
	cs.mocked = true
	assert.Len(t, gc.GetPeers(), 0)

	// Now recreate gc and corrupt the MAC
	// and ensure that the StateInfo message doesn't count
	gc = NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, &joinChanMsg{})
	msg := &receivedMsg{PKIID: pkiIDInOrg1, msg: createStateInfoMsg(1, pkiIDInOrg1, channelA)}
	msg.GetGossipMessage().GetStateInfo().ChannelMAC = GenerateMAC(pkiIDinOrg2, channelA)
	gc.HandleMessage(msg)
	assert.Len(t, gc.GetPeers(), 0)
}

func TestOnDemandGossip(t *testing.T) {
	t.Parallel()

	// Scenario: update the metadata and ensure only 1 dissemination
	// takes place when membership is not empty

	cs := &cryptoService{}
	adapter := new(gossipAdapterMock)
	configureAdapter(adapter)

	gossipedEvents := make(chan struct{})

	conf := conf
	conf.PublishStateInfoInterval = time.Millisecond * 200
	adapter.On("GetConf").Return(conf)
	adapter.On("GetMembership").Return([]discovery.NetworkMember{})
	adapter.On("Gossip", mock.Anything).Run(func(mock.Arguments) {
		gossipedEvents <- struct{}{}
	})
	gc := NewGossipChannel(pkiIDInOrg1, orgInChannelA, cs, channelA, adapter, api.JoinChannelMessage(&joinChanMsg{}))
	defer gc.Stop()
	select {
	case <-gossipedEvents:
		assert.Fail(t, "Should not have gossiped because metadata has not been updated yet")
	case <-time.After(time.Millisecond * 500):
	}
	gc.UpdateStateInfo(createStateInfoMsg(0, pkiIDInOrg1, channelA))
	select {
	case <-gossipedEvents:
	case <-time.After(time.Second):
		assert.Fail(t, "Didn't gossip within a timely manner")
	}
	select {
	case <-gossipedEvents:
	case <-time.After(time.Second):
		assert.Fail(t, "Should have gossiped a second time, because membership is empty")
	}
	adapter = new(gossipAdapterMock)
	configureAdapter(adapter, []discovery.NetworkMember{{}}...)
	adapter.On("Gossip", mock.Anything).Run(func(mock.Arguments) {
		gossipedEvents <- struct{}{}
	})
	gc.(*gossipChannel).Adapter = adapter
	select {
	case <-gossipedEvents:
	case <-time.After(time.Second):
		assert.Fail(t, "Should have gossiped a third time")
	}
	select {
	case <-gossipedEvents:
		assert.Fail(t, "Should not have gossiped a fourth time, because dirty flag should have been turned off")
	case <-time.After(time.Millisecond * 500):
	}
	gc.UpdateStateInfo(createStateInfoMsg(1, pkiIDInOrg1, channelA))
	select {
	case <-gossipedEvents:
	case <-time.After(time.Second):
		assert.Fail(t, "Should have gossiped a block now, because got a new StateInfo message")
	}
}

func createDataUpdateMsg(nonce uint64, seqs ...uint64) *proto.SignedGossipMessage {
	msg := &proto.GossipMessage{
		Nonce:   0,
		Channel: []byte(channelA),
		Tag:     proto.GossipMessage_CHAN_AND_ORG,
		Content: &proto.GossipMessage_DataUpdate{
			DataUpdate: &proto.DataUpdate{
				MsgType: proto.PullMsgType_BLOCK_MSG,
				Nonce:   nonce,
				Data:    []*proto.Envelope{},
			},
		},
	}
	for _, seq := range seqs {
		msg.GetDataUpdate().Data = append(msg.GetDataUpdate().Data, createDataMsg(seq, channelA).Envelope)
	}
	return (msg).NoopSign()
}

func createHelloMsg(PKIID common.PKIidType) *receivedMsg {
	msg := &proto.GossipMessage{
		Channel: []byte(channelA),
		Tag:     proto.GossipMessage_CHAN_AND_ORG,
		Content: &proto.GossipMessage_Hello{
			Hello: &proto.GossipHello{
				Nonce:    500,
				Metadata: nil,
				MsgType:  proto.PullMsgType_BLOCK_MSG,
			},
		},
	}
	return &receivedMsg{msg: msg.NoopSign(), PKIID: PKIID}
}

func dataMsgOfChannel(seqnum uint64, channel common.ChainID) *proto.SignedGossipMessage {
	return (&proto.GossipMessage{
		Channel: []byte(channel),
		Nonce:   0,
		Tag:     proto.GossipMessage_CHAN_AND_ORG,
		Content: &proto.GossipMessage_DataMsg{
			DataMsg: &proto.DataMessage{
				Payload: &proto.Payload{
					Data:   []byte{},
					SeqNum: seqnum,
				},
			},
		},
	}).NoopSign()
}

func createStateInfoMsg(ledgerHeight int, pkiID common.PKIidType, channel common.ChainID) *proto.SignedGossipMessage {
	return (&proto.GossipMessage{
		Tag: proto.GossipMessage_CHAN_OR_ORG,
		Content: &proto.GossipMessage_StateInfo{
			StateInfo: &proto.StateInfo{
				ChannelMAC: GenerateMAC(pkiID, channel),
				Timestamp:  &proto.PeerTime{IncNumber: uint64(time.Now().UnixNano()), SeqNum: 1},
				Metadata:   []byte(fmt.Sprintf("%d", ledgerHeight)),
				PkiId:      []byte(pkiID),
			},
		},
	}).NoopSign()
}

func stateInfoSnapshotForChannel(chainID common.ChainID, stateInfoMsgs ...*proto.SignedGossipMessage) *proto.SignedGossipMessage {
	envelopes := make([]*proto.Envelope, len(stateInfoMsgs))
	for i, sim := range stateInfoMsgs {
		envelopes[i] = sim.Envelope
	}
	return (&proto.GossipMessage{
		Channel: chainID,
		Tag:     proto.GossipMessage_CHAN_OR_ORG,
		Nonce:   0,
		Content: &proto.GossipMessage_StateSnapshot{
			StateSnapshot: &proto.StateInfoSnapshot{
				Elements: envelopes,
			},
		},
	}).NoopSign()
}

func createDataMsg(seqnum uint64, channel common.ChainID) *proto.SignedGossipMessage {
	return (&proto.GossipMessage{
		Nonce:   0,
		Tag:     proto.GossipMessage_CHAN_AND_ORG,
		Channel: []byte(channel),
		Content: &proto.GossipMessage_DataMsg{
			DataMsg: &proto.DataMessage{
				Payload: &proto.Payload{
					Data:   []byte{},
					SeqNum: seqnum,
				},
			},
		},
	}).NoopSign()
}

func simulatePullPhase(gc GossipChannel, t *testing.T, wg *sync.WaitGroup, mutator msgMutator, seqs ...uint64) func(args mock.Arguments) {
	var l sync.Mutex
	var sentHello bool
	var sentReq bool
	return func(args mock.Arguments) {
		msg := args.Get(0).(*proto.SignedGossipMessage)
		l.Lock()
		defer l.Unlock()
		if msg.IsHelloMsg() && !sentHello {
			sentHello = true
			// Simulate a digest message an imaginary peer responds to the hello message sent
			digestMsg := &receivedMsg{
				PKIID: pkiIDInOrg1,
				msg: (&proto.GossipMessage{
					Tag:     proto.GossipMessage_CHAN_AND_ORG,
					Channel: []byte(channelA),
					Content: &proto.GossipMessage_DataDig{
						DataDig: &proto.DataDigest{
							MsgType: proto.PullMsgType_BLOCK_MSG,
							Digests: []string{"10", "11"},
							Nonce:   msg.GetHello().Nonce,
						},
					},
				}).NoopSign(),
			}
			go gc.HandleMessage(digestMsg)
		}
		if msg.IsDataReq() && !sentReq {
			sentReq = true
			dataReq := msg.GetDataReq()
			for _, expectedDigest := range []string{"10", "11"} {
				assert.Contains(t, dataReq.Digests, expectedDigest)
			}
			assert.Equal(t, 2, len(dataReq.Digests))
			// When we send a data request, simulate a response of a data update
			// from the imaginary peer that got the request
			dataUpdateMsg := new(receivedMsg)
			dataUpdateMsg.PKIID = pkiIDInOrg1
			dataUpdateMsg.msg = createDataUpdateMsg(dataReq.Nonce, seqs...)
			mutator(dataUpdateMsg.msg.GetDataUpdate().Data[0])
			gc.HandleMessage(dataUpdateMsg)
			wg.Done()
		}
	}
}

func sequence(start uint64, end uint64) []uint64 {
	sequence := make([]uint64, end-start+1)
	i := 0
	for n := start; n <= end; n++ {
		sequence[i] = n
		i++
	}
	return sequence

}
