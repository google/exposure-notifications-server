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

// This tool attempts to sign a string with all configured export signing keys
// in the system.
package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/internal/buildinfo"
	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/sethvargo/go-signalcontext"
)

var (
	messageToSign = flag.String("message", "hello world", "string message to sign")
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	debug, _ := strconv.ParseBool(os.Getenv("LOG_DEBUG"))
	logger := logging.NewLogger(debug).Named("tools.sign")
	logger = logger.With("build_id", buildinfo.BuildID)
	logger = logger.With("build_tag", buildinfo.BuildTag)

	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	flag.Parse()
	if *messageToSign == "" {
		return fmt.Errorf("--message is required and cannot be empty")
	}

	logger := logging.FromContext(ctx)

	var config export.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	exportDB := database.New(env.Database())
	allSigInfos, err := exportDB.ListAllSignatureInfos(ctx)
	if err != nil {
		return fmt.Errorf("unable to list signature infos, %w", err)
	}

	digest := sha256.Sum256([]byte(*messageToSign))
	logger.Infow("digest to sign", "digest", base64.StdEncoding.EncodeToString(digest[:]))

	now := time.Now().UTC()
	for _, sigInfo := range allSigInfos {
		if !sigInfo.EndTimestamp.IsZero() && now.After(sigInfo.EndTimestamp) {
			logger.Warnw("skipping expired signing key", "kid", sigInfo.SigningKeyID, "version", sigInfo.SigningKeyVersion, "expiry", sigInfo.FormattedEndTimestamp())
			continue
		}

		signer, err := env.GetSignerForKey(ctx, sigInfo.SigningKey)
		if err != nil {
			logger.Errorw("error accessing signing key", "sigInfo", sigInfo, "error", err)
			continue
		}

		sig, err := signer.Sign(rand.Reader, digest[:], crypto.SHA256)
		if err != nil {
			logger.Errorw("error signing message", "sigInfo", sigInfo, "error", err)
			continue
		}

		b64Signature := base64.StdEncoding.EncodeToString(sig)
		logger.Infow("signature", "sigInfo", sigInfo, "message", *messageToSign, "signature", b64Signature)
	}

	return nil
}
