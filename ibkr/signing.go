package ibkr

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

func percentEncode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

// Preserve the Rust/IBKR base-string convention: merge raw parameters, then encode the joined string.
// JSON request bodies are not included in the OAuth signature parameters.
func signatureBase(method, endpoint string, oauth map[string]string, query url.Values, prepend string) string {
	params := map[string]string{}
	for key, value := range oauth {
		params[key] = value
	}
	for key, values := range query {
		if len(values) > 0 {
			params[key] = values[len(values)-1]
		}
	}
	keys := sortedKeys(params)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+params[key])
	}
	return prepend + strings.ToUpper(method) + "&" + percentEncode(endpoint) + "&" + percentEncode(strings.Join(pairs, "&"))
}
func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func oauthParams(config Config, algorithm string) (map[string]string, error) {
	nonce, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	return map[string]string{"oauth_consumer_key": config.ConsumerKey, "oauth_token": config.AccessToken, "oauth_nonce": nonce, "oauth_timestamp": fmt.Sprint(time.Now().Unix()), "oauth_signature_method": algorithm}, nil
}
func authorizationHeader(realm string, params map[string]string) string {
	copied := map[string]string{"realm": realm}
	for key, value := range params {
		copied[key] = value
	}
	pairs := make([]string, 0, len(copied))
	for _, key := range sortedKeys(copied) {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, key, copied[key]))
	}
	return "OAuth " + strings.Join(pairs, ", ")
}
func signProtected(method, endpoint string, query url.Values, token string, config Config) (string, error) {
	params, err := oauthParams(config, "HMAC-SHA256")
	if err != nil {
		return "", err
	}
	key, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("invalid live session token base64")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signatureBase(method, endpoint, params, query, "")))
	params["oauth_signature"] = percentEncode(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	return authorizationHeader(config.Realm, params), nil
}
