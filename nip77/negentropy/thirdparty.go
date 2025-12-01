package negentropy

import (
	"bytes"
	"fmt"

	"fiatjaf.com/nostr"
)

type ThirdPartyNegentropy struct {
	PeerA  NegentropyThirdPartyRemote
	PeerB  NegentropyThirdPartyRemote
	Filter nostr.Filter

	Deltas chan Delta
}

type Delta struct {
	ID      nostr.ID
	Have    NegentropyThirdPartyRemote
	HaveNot NegentropyThirdPartyRemote
}

type boundKey string

func (b Bound) key() boundKey {
	return boundKey(fmt.Sprintf("%d:%x", b.Timestamp, b.IDPrefix))
}

type NegentropyThirdPartyRemote interface {
	SendInitialMessage(filter nostr.Filter, msg string) error
	SendMessage(msg string) error
	SendClose() error
	Receive() (string, error)
}

func NewThirdPartyNegentropy(peerA, peerB NegentropyThirdPartyRemote, filter nostr.Filter) *ThirdPartyNegentropy {
	return &ThirdPartyNegentropy{
		PeerA:  peerA,
		PeerB:  peerB,
		Filter: filter,
		Deltas: make(chan Delta, 100),
	}
}

func (n *ThirdPartyNegentropy) Start() error {
	peerAIds := make(map[nostr.ID]struct{})
	peerBIds := make(map[nostr.ID]struct{})
	peerASkippedBounds := make(map[boundKey]struct{})
	peerBSkippedBounds := make(map[boundKey]struct{})

	// send an empty message to A to start things up
	initialMsg := createInitialMessage()
	err := n.PeerA.SendInitialMessage(n.Filter, initialMsg)
	if err != nil {
		return err
	}

	hasSentInitialMessageToB := false

	for {
		// receive message from A
		msgA, err := n.PeerA.Receive()
		if err != nil {
			return err
		}
		msgAb, _ := nostr.HexDecodeString(msgA)
		if len(msgAb) == 1 {
			break
		}

		msgToB, err := parseMessageBuildNext(
			msgA,
			peerBSkippedBounds,
			func(id nostr.ID) {
				if _, exists := peerBIds[id]; exists {
					delete(peerBIds, id)
				} else {
					peerAIds[id] = struct{}{}
				}
			},
			func(boundKey boundKey) {
				peerASkippedBounds[boundKey] = struct{}{}
			},
		)
		if err != nil {
			return err
		}

		// emit deltas from B after receiving message from A
		for id := range peerBIds {
			n.Deltas <- Delta{ID: id, Have: n.PeerB, HaveNot: n.PeerA}
			delete(peerBIds, id)
		}

		if len(msgToB) == 2 {
			// exit condition (no more messages to send)
			break
		}

		// send message to B
		if hasSentInitialMessageToB {
			err = n.PeerB.SendMessage(msgToB)
		} else {
			err = n.PeerB.SendInitialMessage(n.Filter, msgToB)
			hasSentInitialMessageToB = true
		}
		if err != nil {
			return err
		}

		// receive message from B
		msgB, err := n.PeerB.Receive()
		if err != nil {
			return err
		}
		msgBb, _ := nostr.HexDecodeString(msgB)
		if len(msgBb) == 1 {
			break
		}

		msgToA, err := parseMessageBuildNext(
			msgB,
			peerASkippedBounds,
			func(id nostr.ID) {
				if _, exists := peerAIds[id]; exists {
					delete(peerAIds, id)
				} else {
					peerBIds[id] = struct{}{}
				}
			},
			func(boundKey boundKey) {
				peerBSkippedBounds[boundKey] = struct{}{}
			},
		)
		if err != nil {
			return err
		}

		// emit deltas from A after receiving message from B
		for id := range peerAIds {
			n.Deltas <- Delta{ID: id, Have: n.PeerA, HaveNot: n.PeerB}
			delete(peerAIds, id)
		}

		if len(msgToA) == 2 {
			// exit condition (no more messages to send)
			break
		}

		// send message to A
		err = n.PeerA.SendMessage(msgToA)
		if err != nil {
			return err
		}
	}

	// emit remaining deltas before exit
	for id := range peerAIds {
		n.Deltas <- Delta{ID: id, Have: n.PeerA, HaveNot: n.PeerB}
	}
	for id := range peerBIds {
		n.Deltas <- Delta{ID: id, Have: n.PeerB, HaveNot: n.PeerA}
	}

	n.PeerA.SendClose()
	n.PeerB.SendClose()
	close(n.Deltas)

	return nil
}

func createInitialMessage() string {
	output := bytes.NewBuffer(make([]byte, 0, 64))
	output.WriteByte(protocolVersion)
	writeVarInt(output, 0) // timestamp for infinite
	writeVarInt(output, 0) // prefix len
	output.WriteByte(byte(IdListMode))
	writeVarInt(output, 0) // num ids
	return nostr.HexEncodeToString(output.Bytes())
}

func parseMessageBuildNext(
	msg string,
	skippedBounds map[boundKey]struct{},
	idCallback func(id nostr.ID),
	skipCallback func(boundKey boundKey),
) (next string, err error) {
	msgb, err := nostr.HexDecodeString(msg)
	if err != nil {
		return "", err
	}

	dummy := &Negentropy{}
	nextMsg := bytes.NewBuffer(make([]byte, 0, len(msgb)))
	dummy32BytePlaceholder := [32]byte{}

	reader := bytes.NewReader(msgb)
	pv, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	if pv != protocolVersion {
		return "", fmt.Errorf("unsupported protocol version %v", pv)
	}

	nextMsg.WriteByte(pv)

	for reader.Len() > 0 {
		bound, err := dummy.readBound(reader)
		if err != nil {
			return "", err
		}

		modeVal, err := readVarInt(reader)
		if err != nil {
			return "", err
		}
		mode := Mode(modeVal)

		if _, skipped := skippedBounds[bound.key()]; !skipped {
			dummy.writeBound(nextMsg, bound)
			writeVarInt(nextMsg, modeVal)
		}

		switch mode {
		case SkipMode:
			skipCallback(bound.key())
		case FingerprintMode:
			_, err = reader.Read(dummy32BytePlaceholder[:])
			if err != nil {
				return "", err
			}

			if _, skipped := skippedBounds[bound.key()]; !skipped {
				nextMsg.Write(dummy32BytePlaceholder[:])
			}
		case IdListMode:
			skipCallback(bound.key())

			numIds, err := readVarInt(reader)
			if err != nil {
				return "", err
			}

			if _, skipped := skippedBounds[bound.key()]; !skipped {
				writeVarInt(nextMsg, numIds)
			}

			for range numIds {
				_, err = reader.Read(dummy32BytePlaceholder[:])
				if err != nil {
					return "", err
				}

				idCallback(dummy32BytePlaceholder)

				if _, skipped := skippedBounds[bound.key()]; !skipped {
					nextMsg.Write(dummy32BytePlaceholder[:])
				}
			}
		default:
			return "", fmt.Errorf("unknown mode %v", mode)
		}
	}

	return nostr.HexEncodeToString(nextMsg.Bytes()), nil
}
