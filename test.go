package main

 import (
 	"crypto/ecdsa"
 	"crypto/elliptic"
 	"crypto/md5"
 	"crypto/rand"
 	"fmt"
 	"hash"
 	"io"
 	"math/big"
	 "os"
	 "crypto/x509"
	 "encoding/pem"
	 "io/ioutil"
 )

 func encode(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) (string, string) {
    x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
    pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

    x509EncodedPub, _ := x509.MarshalPKIXPublicKey(publicKey)
    pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

    return string(pemEncoded), string(pemEncodedPub)
}

 func decode(pemEncoded string, pemEncodedPub string) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
    block, _ := pem.Decode([]byte(pemEncoded))
    x509Encoded := block.Bytes
    privateKey, _ := x509.ParseECPrivateKey(x509Encoded)

    blockPub, _ := pem.Decode([]byte(pemEncodedPub))
    x509EncodedPub := blockPub.Bytes
    genericPublicKey, _ := x509.ParsePKIXPublicKey(x509EncodedPub)
    publicKey := genericPublicKey.(*ecdsa.PublicKey)

    return privateKey, publicKey
}

func generateKeys() {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    publicKey := &privateKey.PublicKey

    encPriv, encPub := encode(privateKey, publicKey)

	ioutil.WriteFile("exposure-key.pub", []byte(encPub), 0600)
	ioutil.WriteFile("exposure-key.priv", []byte(encPriv), 0600)

    fmt.Println(encPriv)
    fmt.Println(encPub)

}

 func main() {

	generateKeys()

	encPubb, _ := ioutil.ReadFile("exposure-key.pub")

	encPrivb, _ := ioutil.ReadFile("exposure-key.priv")

	encPub := string(encPubb)
	encPriv := string(encPrivb)
    priv2, pub2 := decode(encPriv, encPub)


 	// Sign ecdsa style

 	var h hash.Hash
 	h = md5.New()
 	r := big.NewInt(0)
 	s := big.NewInt(0)

 	io.WriteString(h, "This is a message to be signed and verified by ECDSA!")
	signhash := h.Sum(nil)
	 
 	r, s, serr := ecdsa.Sign(rand.Reader, priv2, signhash)
 	if serr != nil {
 		fmt.Println(serr)
 		os.Exit(1)
 	}

 	signature := r.Bytes()
 	signature = append(signature, s.Bytes()...)

 	fmt.Printf("Signature : %x\n", signature)

 	// Verify
 	verifystatus := ecdsa.Verify(pub2, signhash, r, s)
 	fmt.Println(verifystatus) // should be true
 }
