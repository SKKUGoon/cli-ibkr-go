package ibkr

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

type liveSession struct {
	Token        string `json:"token"`
	ExpirationMS int64  `json:"expiration_ms"`
	CreatedAtMS  int64  `json:"created_at_ms"`
}

func (session liveSession) valid(skew time.Duration) bool {
	return session.Token != "" && session.ExpirationMS-skew.Milliseconds() > time.Now().UnixMilli()
}
func computeSessionToken(prime, exponent *big.Int, peerHex string, prepend []byte) (string, error) {
	peer, ok := new(big.Int).SetString(peerHex, 16)
	if !ok || peer.Sign() <= 0 || peer.Cmp(prime) >= 0 {
		return "", fmt.Errorf("invalid DH response")
	}
	shared := new(big.Int).Exp(peer, exponent, prime).Bytes()
	// IBKR uses Java BigInteger.toByteArray(), including its positive sign byte.
	if len(shared) == 0 || shared[0]&0x80 != 0 {
		shared = append([]byte{0}, shared...)
	}
	mac := hmac.New(sha1.New, shared)
	mac.Write(prepend)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (client *Client) requestLiveSession(ctx context.Context) (liveSession, error) {
	config := client.config
	dh, err := readDHParameters(config.DHParamPath)
	if err != nil {
		return liveSession{}, err
	}
	randomBytes := make([]byte, 32)
	if _, err = rand.Read(randomBytes); err != nil {
		return liveSession{}, err
	}
	exponent := new(big.Int).SetBytes(randomBytes)
	encryptionKey, err := readPrivateKey(config.EncryptionKeyPath)
	if err != nil {
		return liveSession{}, err
	}
	encrypted, err := base64.StdEncoding.DecodeString(config.AccessTokenSecret)
	if err != nil {
		return liveSession{}, fmt.Errorf("invalid access token secret base64")
	}
	prepend, err := rsa.DecryptPKCS1v15(rand.Reader, encryptionKey, encrypted)
	if err != nil {
		return liveSession{}, fmt.Errorf("access token secret decrypt failed: %w", err)
	}
	params, err := oauthParams(config, "RSA-SHA256")
	if err != nil {
		return liveSession{}, err
	}
	params["diffie_hellman_challenge"] = new(big.Int).Exp(dh.Generator, exponent, dh.Prime).Text(16)
	endpoint := config.endpointURL("oauth/live_session_token")
	digest := sha256.Sum256([]byte(signatureBase("POST", endpoint, params, nil, hex.EncodeToString(prepend))))
	signatureKey, err := readPrivateKey(config.SignatureKeyPath)
	if err != nil {
		return liveSession{}, err
	}
	signature, err := rsa.SignPKCS1v15(rand.Reader, signatureKey, crypto.SHA256, digest[:])
	if err != nil {
		return liveSession{}, err
	}
	params["oauth_signature"] = percentEncode(base64.StdEncoding.EncodeToString(signature))
	raw, err := client.sendJSON(ctx, "live-session-token", "POST", endpoint, nil, nil, authorizationHeader(config.Realm, params))
	if err != nil {
		return liveSession{}, err
	}
	var response struct {
		Peer       string `json:"diffie_hellman_response"`
		Signature  string `json:"live_session_token_signature"`
		Expiration int64  `json:"live_session_token_expiration"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return liveSession{}, err
	}
	token, err := computeSessionToken(dh.Prime, exponent, response.Peer, prepend)
	if err != nil {
		return liveSession{}, err
	}
	key, _ := base64.StdEncoding.DecodeString(token)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(config.ConsumerKey))
	expected, err := hex.DecodeString(response.Signature)
	if err != nil || !hmac.Equal(mac.Sum(nil), expected) {
		return liveSession{}, fmt.Errorf("live session token validation failed")
	}
	return liveSession{token, response.Expiration, time.Now().UnixMilli()}, nil
}
