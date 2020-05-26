// Package signing defines the interface to and implementation of signing
package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"encoding/asn1"
	"math/big"
)

// GCPKMS implements the signing.KeyManager interface and can be used to sign
// export files.
type LocalSigner struct{}

func NewLocalSigner() (KeyManager, error) {
	return &LocalSigner{}, nil
}

func (ls *LocalSigner) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	return &LocalSigner{}, nil
}

func (ls *LocalSigner) Public() crypto.PublicKey {
	pubkey, _ := ReadPublicKeyFromFile()
	return string(pubkey)
}

func ReadPublicKeyFromFile() (string, error) {
	file, ok := os.LookupEnv("SIGN_PUBLIC_KEY_FILE")
	if !ok {
		file = "exposure-uy.pub"
	}
	encPubb, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(encPubb), nil
}

func (ls *LocalSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	file, ok := os.LookupEnv("SIGN_PRIVATE_KEY_FILE")
	if !ok {
		file = "exposure-uy.priv"
	}
	encPrivb, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	encPub, err := ReadPublicKeyFromFile()
	if err != nil {
		return nil, err
	}
	encPriv := string(encPrivb)
	priv2, _, err := decode(encPriv, encPub)
	if err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	r, s, err := ecdsa.Sign(rand, priv2, digest[:])
	if err != nil {
		return nil, fmt.Errorf("unable to sign: %w", err)
	}
	asn1Data := []*big.Int{r, s}
	sig , err := asn1.Marshal(asn1Data)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal asn1: %w", err)
	}
	return sig, err
}

func decode(pemEncoded string, pemEncodedPub string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemEncoded))
	x509Encoded := block.Bytes
	privateKey, err := x509.ParseECPrivateKey(x509Encoded)
	if err != nil {
		return nil, nil, err
	}
	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, err := x509.ParsePKIXPublicKey(x509EncodedPub)
	if err != nil {
		return nil, nil, err
	}
	publicKey := genericPublicKey.(*ecdsa.PublicKey)
	return privateKey, publicKey, nil
}
