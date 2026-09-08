// Package testsupport provides a local OAuth peer. It never connects to IBKR.
package testsupport

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"ibkr-go/ibkr"
)

type Request struct {
	Method, Path string
	Query        url.Values
	Body         json.RawMessage
}
type Server struct {
	URL               string
	Config            ibkr.Config
	Mutex             sync.Mutex
	requests          []Request
	sessions          int
	response          func(Request) (int, string)
	badTokenSignature bool
}

func NewServer(t *testing.T) *Server {
	t.Helper()
	signatureKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encryptionKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	write := func(name, label string, data []byte) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: label, Bytes: data}), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	config := ibkr.DefaultConfig()
	config.ConsumerKey = "test-consumer"
	config.AccessToken = "test-access"
	config.Timeout = 2 * time.Second
	config.SignatureKeyPath = write("signature.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(signatureKey))
	pk8, err := x509.MarshalPKCS8PrivateKey(encryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	config.EncryptionKeyPath = write("encryption.pem", "PRIVATE KEY", pk8)
	prime := big.NewInt(2147483647)
	generator := big.NewInt(5)
	der, err := asn1.Marshal(struct{ Prime, Generator *big.Int }{prime, generator})
	if err != nil {
		t.Fatal(err)
	}
	config.DHParamPath = write("dh.pem", "DH PARAMETERS", der)
	prepend := []byte("synthetic-test-secret")
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, &encryptionKey.PublicKey, prepend)
	if err != nil {
		t.Fatal(err)
	}
	config.AccessTokenSecret = base64.StdEncoding.EncodeToString(encrypted)
	fixture := &Server{Config: config}
	var token []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fixture.Mutex.Lock()
		defer fixture.Mutex.Unlock()
		params := parseAuthorization(request.Header.Get("Authorization"))
		signature, decodeErr := url.QueryUnescape(params["oauth_signature"])
		if decodeErr != nil {
			http.Error(writer, "bad encoding", 401)
			return
		}
		decoded, decodeErr := base64.StdEncoding.DecodeString(signature)
		if decodeErr != nil {
			http.Error(writer, "bad signature", 401)
			return
		}
		delete(params, "oauth_signature")
		delete(params, "realm")
		endpoint := fixture.URL + request.URL.Path
		isSession := strings.HasSuffix(request.URL.Path, "/oauth/live_session_token")
		prefix := ""
		if isSession {
			prefix = hex.EncodeToString(prepend)
		}
		base := referenceBase(request.Method, endpoint, params, request.URL.Query(), prefix)
		if isSession {
			fixture.sessions++
			digest := sha256.Sum256([]byte(base))
			if err := rsa.VerifyPKCS1v15(&signatureKey.PublicKey, crypto.SHA256, digest[:], decoded); err != nil {
				http.Error(writer, "bad RSA signature", 401)
				return
			}
			if request.ContentLength != 0 || request.Header.Get("Content-Length") != "0" {
				http.Error(writer, "missing empty content length", 400)
				return
			}
			challenge, ok := new(big.Int).SetString(params["diffie_hellman_challenge"], 16)
			if !ok {
				http.Error(writer, "bad challenge", 400)
				return
			}
			exponent := big.NewInt(11)
			shared := new(big.Int).Exp(challenge, exponent, prime).Bytes()
			if len(shared) == 0 || shared[0] >= 128 {
				shared = append([]byte{0}, shared...)
			}
			mac := hmac.New(sha1.New, shared)
			mac.Write(prepend)
			token = mac.Sum(nil)
			proof := hmac.New(sha1.New, token)
			proof.Write([]byte(config.ConsumerKey))
			proofHex := hex.EncodeToString(proof.Sum(nil))
			if fixture.badTokenSignature {
				proofHex = "00"
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"diffie_hellman_response": new(big.Int).Exp(generator, exponent, prime).Text(16), "live_session_token_signature": proofHex, "live_session_token_expiration": time.Now().Add(time.Hour).UnixMilli()})
			return
		}
		mac := hmac.New(sha256.New, token)
		mac.Write([]byte(base))
		if !hmac.Equal(mac.Sum(nil), decoded) {
			http.Error(writer, "bad HMAC signature", 401)
			return
		}
		body, _ := io.ReadAll(request.Body)
		entry := Request{request.Method, request.URL.Path, request.URL.Query(), body}
		fixture.requests = append(fixture.requests, entry)
		status, response := 200, `{"ok":true}`
		if fixture.response != nil {
			status, response = fixture.response(entry)
		}
		writer.WriteHeader(status)
		_, _ = io.WriteString(writer, response)
	}))
	fixture.URL = server.URL
	fixture.Config.BaseURL = server.URL + "/v1/api"
	t.Cleanup(server.Close)
	return fixture
}
func parseAuthorization(header string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(header, "OAuth "), ", ") {
		key, value, _ := strings.Cut(part, "=")
		values[key] = strings.Trim(value, `"`)
	}
	return values
}
func referenceBase(method, endpoint string, params map[string]string, query url.Values, prefix string) string {
	for key, values := range query {
		params[key] = values[len(values)-1]
	}
	keys := []string{}
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := []string{}
	for _, key := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, params[key]))
	}
	encode := func(value string) string { return strings.ReplaceAll(url.QueryEscape(value), "+", "%20") }
	return prefix + method + "&" + encode(endpoint) + "&" + encode(strings.Join(pairs, "&"))
}
func (fixture *Server) Environment() map[string]string {
	return map[string]string{
		"IBKR_BASE_URL": fixture.Config.BaseURL, "IBKR_CONSUMER_KEY": fixture.Config.ConsumerKey, "IBKR_REALM": fixture.Config.Realm,
		"IBKR_ACCESS_TOKEN": fixture.Config.AccessToken, "IBKR_ACCESS_TOKEN_SECRET": fixture.Config.AccessTokenSecret,
		"IBKR_SIGNATURE_KEY_PATH": fixture.Config.SignatureKeyPath, "IBKR_ENCRYPTION_KEY_PATH": fixture.Config.EncryptionKeyPath, "IBKR_DH_PARAM_PATH": fixture.Config.DHParamPath,
		"IBKR_LST_CACHE_MODE": "memory", "IBKR_DATABASE": "", "IBKR_ORDERS_ANSWER_JSON": "", "IBKR_TIMEOUT_SECONDS": "2",
	}
}

func (fixture *Server) SnapshotRequests() []Request {
	fixture.Mutex.Lock()
	defer fixture.Mutex.Unlock()
	return append([]Request(nil), fixture.requests...)
}
func (fixture *Server) SessionCount() int {
	fixture.Mutex.Lock()
	defer fixture.Mutex.Unlock()
	return fixture.sessions
}
func (fixture *Server) SetResponse(response func(Request) (int, string)) {
	fixture.Mutex.Lock()
	defer fixture.Mutex.Unlock()
	fixture.response = response
}
func (fixture *Server) SetBadTokenSignature() {
	fixture.Mutex.Lock()
	defer fixture.Mutex.Unlock()
	fixture.badTokenSignature = true
}
