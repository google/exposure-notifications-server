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

// Package pgtest contains helpers to aid testing code that uses PostgreSQL.
package pgtest

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// TODO(dsymonds): This is fairly noisy. Capture output into a log instead?

const (
	superuser = "postgres"
)

// Server represents a hermetic PostgreSQL server.
type Server struct {
	psql    string
	sockDir string

	postgres *exec.Cmd
	ready    chan struct{} // Closed when the server is ready.
	done     chan struct{} // Closed when the process ends.
}

// ErrNoPostgreSQL is returned by NewServer when it fails due to
// PostgreSQL not being installed.
var ErrNoPostgreSQL = errors.New("didn't find a postgres binary; is PostgreSQL installed?")

// NewServer initializes and starts a hermetic PostgreSQL server.
// The server is not necessarily ready to serve when returned;
// use the WaitForReady method if needed.
//
// NewServer returns ErrNoPostgreSQL if PostgreSQL is not installed.
//
// The server creates files under workDir. That directory should be
// deleted by the caller after the Shutdown method is called.
func NewServer(ctx context.Context, workDir string) (*Server, error) {
	// Check that the necessary tools are in the PATH.
	if _, err := exec.LookPath("initdb"); err != nil {
		return nil, ErrNoPostgreSQL
	}
	psql, err := exec.LookPath("psql")
	if err != nil {
		return nil, ErrNoPostgreSQL
	}
	postgresBin, err := exec.LookPath("postgres")
	if err != nil {
		return nil, ErrNoPostgreSQL
	}

	dataDir := filepath.Join(workDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "initdb",
		"--pgdata", dataDir,
		"--encoding", "UTF8",
		"--username", superuser)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("initializing DB: %v", err)
	}

	// Configure host-based auth that permits superuser to have full (password-free) access,
	// and require MD5-based password auth for everyone else.
	const hba = `
local	all	` + superuser + `	trust
local	all	all	md5
	`
	if err := ioutil.WriteFile(filepath.Join(dataDir, "pg_hba.conf"), []byte(hba), 0644); err != nil {
		return nil, err
	}

	sockDir := filepath.Join(workDir, "socket")
	if err := os.MkdirAll(sockDir, 0755); err != nil {
		return nil, err
	}

	postgresArgs := []string{
		"-D", dataDir,
		// Listen only on a Unix domain socket.
		// This is easier than trying to get a free IP port for TCP/IP.
		"-k", sockDir,
		"-h", "",
	}
	cmd = exec.CommandContext(ctx, postgresBin, postgresArgs...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	// Don't attach to the postgres stdout. It doesn't write anything there,
	// but it having a stdout appears to make it start very slowly.
	// Run the command in a separate process group so Ctrl+C won't interrupt it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting postgres: %v", err)
	}

	s := &Server{
		psql:    psql,
		sockDir: sockDir,

		postgres: cmd,
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
	}
	go func() {
		s.postgres.Wait()
		close(s.done)
	}()
	go s.monitor(os.Stderr, stderr)
	return s, nil
}

func (s *Server) monitor(w io.Writer, r io.Reader) {
	scan := bufio.NewScanner(r)
	ready := false
	for scan.Scan() {
		line := scan.Text()

		io.WriteString(w, "postgres: "+line+"\n")

		if !ready && strings.Contains(line, "database system is ready to accept connections") {
			ready = true
			close(s.ready)
		}
	}
}

// WaitForReady blocks until the server is ready to serve,
// or until the context expires. It returns true in the first case,
// and false in the second case.
func (s *Server) WaitForReady(ctx context.Context) bool {
	select {
	case <-s.ready:
		return true
	case <-ctx.Done():
		return false
	}
}

// Shutdown shuts the server down. It blocks until it exits.
func (s *Server) Shutdown() {
	s.postgres.Process.Signal(os.Interrupt)
	<-s.done
}

// Addr returns the address of the server.
// It is suitable for use as the host of a DB connection configuration.
func (s *Server) Addr() string {
	return s.sockDir
}

// CreateUser creates a new database user.
func (s *Server) CreateUser(ctx context.Context, username, password string) error {
	return s.runpsql(ctx, []string{
		"CREATE ROLE " + username + " WITH LOGIN PASSWORD '" + password + "';",
	})
}

// CreateDatabase creates a new database, and grants all privileges on it to the named user.
func (s *Server) CreateDatabase(ctx context.Context, dbName, owner string) error {
	return s.runpsql(ctx, []string{
		"CREATE DATABASE " + dbName + ";",
		"GRANT ALL PRIVILEGES ON DATABASE " + dbName + " TO " + owner + ";",
	})
}

func (s *Server) runpsql(ctx context.Context, lines []string) error {
	var input bytes.Buffer
	for _, line := range lines {
		input.WriteString(line + "\n")
	}

	cmd := exec.CommandContext(ctx, s.psql,
		"-U", superuser,
		"--host", s.sockDir,
		"--no-readline", "--no-password")
	cmd.Stdin = &input
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
