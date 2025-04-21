package nip46

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip44"
	"github.com/mailru/easyjson"
)

var _ Signer = (*DynamicSigner)(nil)

type DynamicSigner struct {
	sessions map[nostr.PubKey]Session

	sync.Mutex

	getHandlerSecretKey func(handlerPubkey nostr.PubKey) ([32]byte, error)
	getUserKeyer        func(handlerPubkey nostr.PubKey) (nostr.Keyer, error)
	authorizeSigning    func(event nostr.Event, from nostr.PubKey, secret string) bool
	authorizeEncryption func(from nostr.PubKey, secret string) bool
	onEventSigned       func(event nostr.Event)
}

func NewDynamicSigner(
	// the handler is the keypair we use to communicate with the NIP-46 client, decrypt requests, encrypt responses etc
	getHandlerSecretKey func(handlerPubkey nostr.PubKey) ([32]byte, error),

	// this should correspond to the actual user on behalf of which we will respond to requests
	getUserKeyer func(handlerPubkey nostr.PubKey) (nostr.Keyer, error),

	// this is called on every sign_event call, if it is nil it will be assumed that everything is authorized
	authorizeSigning func(event nostr.Event, from nostr.PubKey, secret string) bool,

	// this is called on every encrypt or decrypt calls, if it is nil it will be assumed that everything is authorized
	authorizeEncryption func(from nostr.PubKey, secret string) bool,

	// unless it is nil, this is called after every event is signed
	onEventSigned func(event nostr.Event),
) DynamicSigner {
	return DynamicSigner{
		getHandlerSecretKey: getHandlerSecretKey,
		getUserKeyer:        getUserKeyer,
		authorizeSigning:    authorizeSigning,
		authorizeEncryption: authorizeEncryption,
		onEventSigned:       onEventSigned,
	}
}

func (p *DynamicSigner) GetSession(clientPubkey nostr.PubKey) (Session, bool) {
	session, exists := p.sessions[clientPubkey]
	if exists {
		return session, true
	}
	return Session{}, false
}

func (p *DynamicSigner) setSession(clientPubkey nostr.PubKey, session Session) {
	p.Lock()
	defer p.Unlock()

	_, exists := p.sessions[clientPubkey]
	if exists {
		return
	}

	// add to pool
	p.sessions[clientPubkey] = session
}

func (p *DynamicSigner) HandleRequest(ctx context.Context, event nostr.Event) (
	req Request,
	resp Response,
	eventResponse nostr.Event,
	err error,
) {
	if event.Kind != nostr.KindNostrConnect {
		return req, resp, eventResponse,
			fmt.Errorf("event kind is %d, but we expected %d", event.Kind, nostr.KindNostrConnect)
	}

	handler := event.Tags.Find("p")
	if handler == nil || !nostr.IsValid32ByteHex(handler[1]) {
		return req, resp, eventResponse, fmt.Errorf("invalid \"p\" tag")
	}

	handlerPubkey, err := nostr.PubKeyFromHex(handler[1])
	if err != nil {
		return req, resp, eventResponse, fmt.Errorf("%x is invalid pubkey: %w", handler[1], err)
	}
	handlerSecret, err := p.getHandlerSecretKey(handlerPubkey)
	if err != nil {
		return req, resp, eventResponse, fmt.Errorf("no private key for %s: %w", handlerPubkey, err)
	}
	userKeyer, err := p.getUserKeyer(handlerPubkey)
	if err != nil {
		return req, resp, eventResponse, fmt.Errorf("failed to get user keyer for %s: %w", handlerPubkey, err)
	}

	session, exists := p.sessions[event.PubKey]
	if !exists {
		session = Session{}

		session.ConversationKey, err = nip44.GenerateConversationKey(event.PubKey, handlerSecret)
		if err != nil {
			return req, resp, eventResponse, fmt.Errorf("failed to compute shared secret: %w", err)
		}

		session.PublicKey, err = userKeyer.GetPublicKey(ctx)
		if err != nil {
			return req, resp, eventResponse, fmt.Errorf("failed to get public key: %w", err)
		}

		p.setSession(event.PubKey, session)
	}

	req, err = session.ParseRequest(event)
	if err != nil {
		return req, resp, eventResponse, fmt.Errorf("error parsing request: %w", err)
	}

	var secret string
	var result string
	var resultErr error

	switch req.Method {
	case "connect":
		if len(req.Params) >= 2 {
			secret = req.Params[1]
		}
		result = "ack"
	case "get_public_key":
		result = hex.EncodeToString(session.PublicKey[:])
	case "sign_event":
		if len(req.Params) != 1 {
			resultErr = fmt.Errorf("wrong number of arguments to 'sign_event'")
			break
		}
		evt := nostr.Event{}
		err = easyjson.Unmarshal([]byte(req.Params[0]), &evt)
		if err != nil {
			resultErr = fmt.Errorf("failed to decode event/2: %w", err)
			break
		}
		if p.authorizeSigning != nil && !p.authorizeSigning(evt, event.PubKey, secret) {
			resultErr = fmt.Errorf("refusing to sign this event")
			break
		}

		err = userKeyer.SignEvent(ctx, &evt)
		if err != nil {
			resultErr = fmt.Errorf("failed to sign event: %w", err)
			break
		}
		jrevt, _ := easyjson.Marshal(evt)
		result = string(jrevt)
	case "nip44_encrypt":
		if len(req.Params) != 2 {
			resultErr = fmt.Errorf("wrong number of arguments to 'nip44_encrypt'")
			break
		}
		thirdPartyPubkey, err := nostr.PubKeyFromHex(req.Params[0])
		if err != nil {
			resultErr = fmt.Errorf("first argument to 'nip44_encrypt' is not a valid pubkey string")
			break
		}
		if p.authorizeEncryption != nil && !p.authorizeEncryption(event.PubKey, secret) {
			resultErr = fmt.Errorf("refusing to encrypt")
			break
		}
		plaintext := req.Params[1]

		ciphertext, err := userKeyer.Encrypt(ctx, plaintext, thirdPartyPubkey)
		if err != nil {
			resultErr = fmt.Errorf("failed to encrypt: %w", err)
			break
		}
		result = ciphertext
	case "nip44_decrypt":
		if len(req.Params) != 2 {
			resultErr = fmt.Errorf("wrong number of arguments to 'nip04_decrypt'")
			break
		}
		thirdPartyPubkey, err := nostr.PubKeyFromHex(req.Params[0])
		if err != nil {
			resultErr = fmt.Errorf("first argument to 'nip04_decrypt' is not a valid pubkey string")
			break
		}
		if p.authorizeEncryption != nil && !p.authorizeEncryption(event.PubKey, secret) {
			resultErr = fmt.Errorf("refusing to decrypt")
			break
		}
		ciphertext := req.Params[1]

		plaintext, err := userKeyer.Decrypt(ctx, ciphertext, thirdPartyPubkey)
		if err != nil {
			resultErr = fmt.Errorf("failed to decrypt: %w", err)
			break
		}
		result = plaintext
	case "ping":
		result = "pong"
	default:
		return req, resp, eventResponse,
			fmt.Errorf("unknown method '%s'", req.Method)
	}

	resp, eventResponse, err = session.MakeResponse(req.ID, event.PubKey, result, resultErr)
	if err != nil {
		return req, resp, eventResponse, err
	}

	err = eventResponse.Sign(handlerSecret)
	if err != nil {
		return req, resp, eventResponse, err
	}

	return req, resp, eventResponse, err
}
