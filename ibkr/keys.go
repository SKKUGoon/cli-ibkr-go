package ibkr

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
)

func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid private key PEM: %s", path)
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("expected RSA private key: %s", path)
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("expected PKCS#1 or PKCS#8 private key: %s", path)
	}
}

type dhParameters struct {
	Prime, Generator *big.Int
	PrivateLength    int `asn1:"optional"`
}

func readDHParameters(path string) (dhParameters, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dhParameters{}, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return dhParameters{}, fmt.Errorf("missing DH PARAMETERS PEM")
	}
	var params dhParameters
	rest, err := asn1.Unmarshal(block.Bytes, &params)
	if err != nil || len(rest) != 0 || params.Prime == nil || params.Generator == nil {
		return params, fmt.Errorf("invalid DH parameters")
	}
	if params.Prime.Sign() <= 0 || params.Generator.Sign() <= 0 || params.Generator.Cmp(params.Prime) >= 0 {
		return params, fmt.Errorf("invalid DH prime or generator")
	}
	return params, nil
}

// ValidateOAuthKeyFiles parses local OAuth materials without contacting IBKR.
func ValidateOAuthKeyFiles(config Config) error {
	for _, path := range []string{config.SignatureKeyPath, config.EncryptionKeyPath} {
		key, err := readPrivateKey(path)
		if err != nil {
			return err
		}
		if err := key.Validate(); err != nil {
			return fmt.Errorf("invalid RSA key: %s", path)
		}
	}
	_, err := readDHParameters(config.DHParamPath)
	return err
}
