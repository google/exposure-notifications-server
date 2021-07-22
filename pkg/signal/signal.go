// Copyright 2020 the Exposure Notifications Server authors
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

// Package signal provides convenance methods for sending interrupt signals to the self process.
package signal

import (
	"fmt"
	"os"
)

// SendInterrupt sends an interrupt to the current process.
func SendInterrupt() error {
	pid := os.Getpid()
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("unable to find own pid: %w", err)
	}

	err = proc.Signal(os.Interrupt)
	if err != nil {
		return fmt.Errorf("signal: %w", err)
	}
	return nil
}
