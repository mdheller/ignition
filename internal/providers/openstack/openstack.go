// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The OpenStack provider fetches configurations from the userdata available in
// both the config-drive as well as the network metadata service. Whichever
// responds first is the config that is used.
// NOTE: This provider is still EXPERIMENTAL.

package openstack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/ignition/config/validate/report"
	"github.com/coreos/ignition/internal/config"
	"github.com/coreos/ignition/internal/config/types"
	"github.com/coreos/ignition/internal/distro"
	"github.com/coreos/ignition/internal/log"
	"github.com/coreos/ignition/internal/resource"
)

const (
	configDriveUserdataPath = "/openstack/latest/user_data"
)

var (
	metadataServiceUrl = url.URL{
		Scheme: "http",
		Host:   "169.254.169.254",
		Path:   "openstack/latest/user_data",
	}
	ErrNoBoundary      = errors.New("found multipart message but no boundary; could not parse")
	ErrMultipleConfigs = errors.New("found multiple configs in multipart response")
)

func FetchConfig(f *resource.Fetcher) (types.Config, report.Report, error) {
	var data []byte
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	dispatch := func(name string, fn func() ([]byte, error)) {
		raw, err := fn()
		if err != nil {
			switch err {
			case context.Canceled:
			case context.DeadlineExceeded:
				f.Logger.Err("timed out while fetching config from %s", name)
			default:
				f.Logger.Err("failed to fetch config from %s: %v", name, err)
			}
			return
		}

		data = raw
		cancel()
	}

	go dispatch("config drive (config-2)", func() ([]byte, error) {
		return fetchConfigFromDevice(f.Logger, ctx, filepath.Join(distro.DiskByLabelDir(), "config-2"))
	})

	go dispatch("config drive (CONFIG-2)", func() ([]byte, error) {
		return fetchConfigFromDevice(f.Logger, ctx, filepath.Join(distro.DiskByLabelDir(), "CONFIG-2"))
	})

	go dispatch("metadata service", func() ([]byte, error) {
		return fetchConfigFromMetadataService(f)
	})

	<-ctx.Done()
	if ctx.Err() == context.DeadlineExceeded {
		f.Logger.Info("neither config drive nor metadata service were available in time. Continuing without a config...")
	}

	return config.Parse(data)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return (err == nil)
}

func fetchConfigFromDevice(logger *log.Logger, ctx context.Context, path string) ([]byte, error) {
	for !fileExists(path) {
		logger.Debug("config drive (%q) not found. Waiting...", path)
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	logger.Debug("creating temporary mount point")
	mnt, err := ioutil.TempDir("", "ignition-configdrive")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.Remove(mnt)

	cmd := exec.Command(distro.MountCmd(), "-o", "ro", "-t", "auto", path, mnt)
	if _, err := logger.LogCmd(cmd, "mounting config drive"); err != nil {
		return nil, err
	}
	defer logger.LogOp(
		func() error { return syscall.Unmount(mnt, 0) },
		"unmounting %q at %q", path, mnt,
	)

	if !fileExists(filepath.Join(mnt, configDriveUserdataPath)) {
		return nil, nil
	}

	return ioutil.ReadFile(filepath.Join(mnt, configDriveUserdataPath))
}

func fetchConfigFromMetadataService(f *resource.Fetcher) ([]byte, error) {
	var ResponseHeaders http.Header
	res, err := f.FetchToBuffer(metadataServiceUrl, resource.FetchOptions{
		Headers:         resource.ConfigHeaders,
		ResponseHeaders: &ResponseHeaders,
	})
	if err != nil {
		return nil, err
	}
	// Openstack with Heat returns the Ignition config as a section of a multipart sequence.
	// Detect if we got a multipart message, ensure that there is only one Ignition config,
	// and extract it.
	mediaType, params, err := mime.ParseMediaType(ResponseHeaders.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		// either unset or not multipart/mixed, just return the blob
		// we don't require proper Content-Type headers
		return res, nil
	}
	boundary, ok := params["boundary"]
	if !ok {
		return nil, ErrNoBoundary
	}
	mpReader := multipart.NewReader(bytes.NewReader(res), boundary)
	var ignConfig []byte
	for {
		part, err := mpReader.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		partType := part.Header.Get("Content-Type")
		if strings.HasPrefix(partType, "application/vnd.coreos.ignition+json") {
			if ignConfig != nil {
				// found more than one ignition config, die.
				return nil, ErrMultipleConfigs
			}
			ignConfig, err = ioutil.ReadAll(part)
			if err != nil {
				return nil, err
			}
		}
	}
	return ignConfig, nil
}
