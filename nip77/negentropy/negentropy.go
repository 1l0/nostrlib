package negentropy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"unsafe"

	"fiatjaf.com/nostr"
)

const (
	protocolVersion byte = 0x61 // version 1
	maxTimestamp         = nostr.Timestamp(math.MaxInt64)
	buckets              = 16
)

var InfiniteBound = Bound{Timestamp: maxTimestamp}

type Negentropy struct {
	storage          Storage
	initialized      bool
	frameSizeLimit   int
	isClient         bool
	lastTimestampIn  nostr.Timestamp
	lastTimestampOut nostr.Timestamp

	Haves    chan nostr.ID
	HaveNots chan nostr.ID
}

func New(storage Storage, frameSizeLimit int) *Negentropy {
	if frameSizeLimit == 0 {
		frameSizeLimit = math.MaxInt
	} else if frameSizeLimit < 4096 {
		panic(fmt.Errorf("frameSizeLimit can't be smaller than 4096, was %d", frameSizeLimit))
	}

	return &Negentropy{
		storage:        storage,
		frameSizeLimit: frameSizeLimit,
		Haves:          make(chan nostr.ID, buckets*4),
		HaveNots:       make(chan nostr.ID, buckets*4),
	}
}

func (n *Negentropy) String() string {
	label := "uninitialized"
	if n.initialized {
		label = "server"
		if n.isClient {
			label = "client"
		}
	}
	return fmt.Sprintf("<Negentropy %s with %d items>", label, n.storage.Size())
}

func (n *Negentropy) Start() string {
	n.initialized = true
	n.isClient = true

	output := bytes.NewBuffer(make([]byte, 0, 1+n.storage.Size()*64))
	output.WriteByte(protocolVersion)
	n.SplitRange(0, n.storage.Size(), InfiniteBound, output)

	return hex.EncodeToString(output.Bytes())
}

func (n *Negentropy) Reconcile(msg string) (string, error) {
	n.initialized = true
	msgb, err := hex.DecodeString(msg)
	if err != nil {
		return "", err
	}

	reader := bytes.NewReader(msgb)

	output, err := n.reconcileAux(reader)
	if err != nil {
		return "", err
	}

	if len(output) == 1 && n.isClient {
		close(n.Haves)
		close(n.HaveNots)
		return "", nil
	}

	return hex.EncodeToString(output), nil
}

func (n *Negentropy) reconcileAux(reader *bytes.Reader) ([]byte, error) {
	n.lastTimestampIn, n.lastTimestampOut = 0, 0 // reset for each message

	fullOutput := bytes.NewBuffer(make([]byte, 0, 5000))
	fullOutput.WriteByte(protocolVersion)

	pv, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read pv: %w", err)
	}
	if pv != protocolVersion {
		if n.isClient {
			return nil, fmt.Errorf("unsupported negentropy protocol version %v", pv)
		}

		// if we're a server we just return our protocol version
		return fullOutput.Bytes(), nil
	}

	var prevBound Bound
	prevIndex := 0
	skipping := false // this means we are currently coalescing ranges into skip

	partialOutput := bytes.NewBuffer(make([]byte, 0, 100))
	for reader.Len() > 0 {
		partialOutput.Reset()

		finishSkip := func() {
			// end skip range, if necessary, so we can start a new bound that isn't a skip
			if skipping {
				skipping = false
				n.writeBound(partialOutput, prevBound)
				partialOutput.WriteByte(byte(SkipMode))
			}
		}

		currBound, err := n.readBound(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode bound: %w", err)
		}
		modeVal, err := readVarInt(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode mode: %w", err)
		}
		mode := Mode(modeVal)

		lower := prevIndex
		upper := n.storage.FindLowerBound(prevIndex, n.storage.Size(), currBound)

		switch mode {
		case SkipMode:
			skipping = true

		case FingerprintMode:
			theirFingerprint := [FingerprintSize]byte{}
			_, err := reader.Read(theirFingerprint[:])
			if err != nil {
				return nil, fmt.Errorf("failed to read fingerprint: %w", err)
			}
			ourFingerprint := n.storage.Fingerprint(lower, upper)

			if theirFingerprint == ourFingerprint {
				skipping = true
			} else {
				finishSkip()
				n.SplitRange(lower, upper, currBound, partialOutput)
			}

		case IdListMode:
			numIds, err := readVarInt(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode number of ids: %w", err)
			}

			// what they have
			theirItems := make(map[nostr.ID]struct{}, numIds)
			for i := 0; i < numIds; i++ {
				var id [32]byte
				if _, err := reader.Read(id[:]); err != nil {
					return nil, fmt.Errorf("failed to read id (#%d/%d) in list: %w", i, numIds, err)
				} else {
					theirItems[id] = struct{}{}
				}
			}

			// what we have
			for _, item := range n.storage.Range(lower, upper) {
				id := item.ID

				if _, theyHave := theirItems[id]; theyHave {
					// if we have and they have, ignore
					delete(theirItems, id)
				} else {
					// if we have and they don't, notify client
					if n.isClient {
						n.Haves <- id
					}
				}
			}

			if n.isClient {
				// notify client of what they have and we don't
				for id := range theirItems {
					// skip empty strings here because those were marked to be excluded as such in the previous step
					n.HaveNots <- id
				}

				// client got list of ids, it's done, skip
				skipping = true
			} else {
				// server got list of ids, reply with their own ids for the same range
				finishSkip()

				responseIds := make([]byte, 0, 32*100)
				responses := 0

				endBound := currBound

				for index, item := range n.storage.Range(lower, upper) {
					if n.frameSizeLimit-200 < fullOutput.Len()/2+len(responseIds)/2 {
						endBound = Bound{item.Timestamp, item.ID[:]}
						upper = index
						break
					}
					responseIds = append(responseIds, item.ID[:]...)
					responses++
				}

				n.writeBound(partialOutput, endBound)
				partialOutput.WriteByte(byte(IdListMode))
				writeVarInt(partialOutput, responses)
				partialOutput.Write(responseIds)

				io.Copy(fullOutput, partialOutput)
				partialOutput.Reset()
			}

		default:
			return nil, fmt.Errorf("unexpected mode %d", mode)
		}

		if n.frameSizeLimit-200 < fullOutput.Len()/2+partialOutput.Len()/2 {
			// frame size limit exceeded, handle by encoding a boundary and fingerprint for the remaining range
			remainingFingerprint := n.storage.Fingerprint(upper, n.storage.Size())
			n.writeBound(fullOutput, InfiniteBound)
			fullOutput.WriteByte(byte(FingerprintMode))
			fullOutput.Write(remainingFingerprint[:])

			break // stop processing further
		} else {
			// append the constructed output for this iteration
			io.Copy(fullOutput, partialOutput)
		}

		prevIndex = upper
		prevBound = currBound
	}

	return fullOutput.Bytes(), nil
}

func (n *Negentropy) SplitRange(lower, upper int, upperBound Bound, output *bytes.Buffer) {
	numElems := upper - lower

	if numElems < buckets*2 {
		// we just send the full ids here
		n.writeBound(output, upperBound)
		output.WriteByte(byte(IdListMode))
		writeVarInt(output, numElems)

		for _, item := range n.storage.Range(lower, upper) {
			output.Write(item.ID[:])
		}
	} else {
		itemsPerBucket := numElems / buckets
		bucketsWithExtra := numElems % buckets
		curr := lower

		for i := 0; i < buckets; i++ {
			bucketSize := itemsPerBucket
			if i < bucketsWithExtra {
				bucketSize++
			}
			ourFingerprint := n.storage.Fingerprint(curr, curr+bucketSize)
			curr += bucketSize

			var nextBound Bound
			if curr == upper {
				nextBound = upperBound
			} else {
				var prevItem, currItem Item

				for index, item := range n.storage.Range(curr-1, curr+1) {
					if index == curr-1 {
						prevItem = item
					} else {
						currItem = item
					}
				}

				minBound := getMinimalBound(prevItem, currItem)
				nextBound = minBound
			}

			n.writeBound(output, nextBound)
			output.WriteByte(byte(FingerprintMode))
			output.Write(ourFingerprint[:])
		}
	}
}

func (n *Negentropy) Name() string {
	p := unsafe.Pointer(n)
	return fmt.Sprintf("%d", uintptr(p)&127)
}
