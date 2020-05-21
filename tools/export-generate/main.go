// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This utility generates test exports signed with local keys
package main

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"time"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/export/model"

	"github.com/google/exposure-notifications-server/internal/util"
)

var (
	signingKey     = flag.String("signing-key", "", "The path to a private key PEM to use for signing")
	keyID          = flag.String("key-id", "some_id", "Value to use in verification_key_id")
	keyVersion     = flag.String("key-version", "1", "Value to use in verification_key_version")
	filenameRoot   = flag.String("filename-root", "/tmp/testExport-", "The root filename for the export file(s).")
	region         = flag.String("region", "US", "The region for the test export.")
	startTimestamp = flag.String("start-timestamp", "2020-05-01T15:00:00Z", "The test export start timestamp (RFC3339).")
	endTimestamp   = flag.String("end-timestamp", "2020-05-02T15:00:00Z", "The test export end timestamp (RFC3339).")
	numKeys        = flag.Int("num-keys", 450, "Number of total random temporary exposure keys to generate. Ignored if tek-file set.")
	tekFile        = flag.String("tek-file", "", "JSON file of TEKs in the same format as calling publish endpoint")
	batchSize      = flag.Int("batches-size", 100, "Max number of keys in each file in the batch")
)

const (
	filenameSuffix = ".zip"
)

func main() {
	flag.Parse()

	if *signingKey == "" {
		log.Fatal("--signing-key is required.")
	}

	startTime := time.Now()
	if *startTimestamp != "" {
		var err error
		startTime, err = time.Parse(time.RFC3339, *startTimestamp)
		if err != nil {
			log.Fatalf("Failed to parse --start-timestamp (use RFC3339): %v", err)
		}
	}

	var endTime time.Time
	if *endTimestamp != "" {
		var err error
		endTime, err = time.Parse(time.RFC3339, *endTimestamp)
		if err != nil {
			log.Fatalf("Failed to parse --end-timestamp (use RFC3339): %v", err)
		}
	}

	// parse signing key
	var privateKey *ecdsa.PrivateKey
	privateKey, err := getSigningKey(*signingKey)
	if err != nil {
		log.Fatalf("unable to generate signing key: %v", err)
	}

	// generate fake keys
	tr, err := util.RandomTransmissionRisk()
	if err != nil {
		log.Fatalf("problem with random transmission risk: %v", err)
	}
	var actualNumKeys int
	var exposureKeys []coredb.Exposure
	if *tekFile != "" {
		log.Printf("Using TEKs provided in: %s", *tekFile)
		file, err := ioutil.ReadFile(*tekFile)
		if err != nil {
			log.Fatalf("unable to read file: %v", err)
		}
		data := coredb.ExposureKeys{}
		err = json.Unmarshal([]byte(file), &data)
		if err != nil {
			log.Fatalf("unable to parse json: %v", err)
		}
		for _, k := range data.Keys {
			ek, err := coredb.TransformExposureKey(k, "", []string{}, time.Now(), int32(0), math.MaxInt32)
			if err != nil {
				log.Fatalf("invalid exposure key: %v", err)
			}
			exposureKeys = append(exposureKeys, *ek)
		}
		actualNumKeys = len(exposureKeys)
	} else {
		log.Printf("Genrating %d random TEKs", *numKeys)
		keys := util.GenerateExposureKeys(*numKeys, tr)
		actualNumKeys = *numKeys

		exposureKeys = make([]coredb.Exposure, actualNumKeys)
		for i, k := range keys {
			decoded, err := base64.StdEncoding.DecodeString(k.Key)
			if err != nil {
				log.Fatalf("unable to decode key: %v", k.Key)
			}
			exposureKeys[i].ExposureKey = decoded
			n, err := util.RandomIntervalCount()
			if err != nil {
				log.Fatalf("problem with interval count: %v", err)
			}
			exposureKeys[i].IntervalNumber = n
			exposureKeys[i].IntervalCount = n
		}
	}

	// split up into batches
	eb := &model.ExportBatch{
		FilenameRoot:   *filenameRoot,
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Region:         *region,
	}
	numBatches := int(math.Ceil(float64(actualNumKeys) / float64(*batchSize)))
	log.Printf("number of batches: %d", numBatches)
	b := 0
	currentBatch := []*coredb.Exposure{}
	for i := 0; i < actualNumKeys; i++ {
		currentBatch = append(currentBatch, &exposureKeys[i])
		if len(currentBatch) == *batchSize {
			b++
			writeFile(eb, currentBatch, b, numBatches, actualNumKeys, privateKey)
			currentBatch = []*coredb.Exposure{}
		}
	}
	if len(currentBatch) > 0 {
		b++
		writeFile(eb, currentBatch, b, numBatches, actualNumKeys, privateKey)
	}
}

func writeFile(eb *model.ExportBatch, currentBatch []*coredb.Exposure, b, numBatches, numRecords int, privateKey *ecdsa.PrivateKey) {
	signatureInfo := &model.SignatureInfo{
		SigningKeyID:      *keyID,
		SigningKeyVersion: *keyVersion,
	}
	signer := export.ExportSigners{
		SignatureInfo: signatureInfo,
		Signer:        privateKey,
	}
	data, err := export.MarshalExportFile(eb, currentBatch, b, numBatches, []export.ExportSigners{signer})
	if err != nil {
		log.Fatalf("error marshalling export file: %v", err)
	}
	fileName := fmt.Sprintf(eb.FilenameRoot+"%d-records-%d-of-%d"+filenameSuffix, numRecords, b, numBatches)
	log.Printf("Creating %v", fileName)
	err = ioutil.WriteFile(fileName, data, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

func getSigningKey(fileName string) (*ecdsa.PrivateKey, error) {
	keyBytes, _ := ioutil.ReadFile(fileName)
	return ParseECPrivateKeyFromPEM(keyBytes)
}

// Parse PEM encoded Elliptic Curve Private Key Structure
func ParseECPrivateKeyFromPEM(key []byte) (*ecdsa.PrivateKey, error) {
	ErrNotECPrivateKey := errors.New("Key is not a valid ECDSA private key")
	ErrKeyMustBePEMEncoded := errors.New("Invalid Key: Key must be PEM encoded PKCS1 or PKCS8 private key")
	var err error

	// Parse PEM block
	var block *pem.Block
	if block, _ = pem.Decode(key); block == nil {
		return nil, ErrKeyMustBePEMEncoded
	}

	// Parse the key
	var parsedKey interface{}
	if parsedKey, err = x509.ParseECPrivateKey(block.Bytes); err != nil {
		return nil, err
	}

	var pkey *ecdsa.PrivateKey
	var ok bool
	if pkey, ok = parsedKey.(*ecdsa.PrivateKey); !ok {
		return nil, ErrNotECPrivateKey
	}

	return pkey, nil
}
