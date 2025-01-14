// Copyright 2021 Red Hat, Inc.
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

package filesystems

import (
	"github.com/flatcar-linux/ignition/v2/tests/register"
	"github.com/flatcar-linux/ignition/v2/tests/types"
)

func init() {
	register.Register(register.NegativeTest, EraseBlockDeviceWithInvalidoptions())
}

// EraseBlockDeviceWithInvalidoptions verifies that the Ignition
// fails to erase the block device with pre-existing filesystem
// on it if `wipeFileSystem is set to `false` and the format is
// set to `none`.
func EraseBlockDeviceWithInvalidoptions() types.Test {
	name := "filesystem.erase.block.device.invalidoptions"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	mntDevices := []types.MntDevice{
		{
			Label:        "EFI-SYSTEM",
			Substitution: "$DEVICE",
		},
	}
	config := `{
		"ignition": { "version": "$version" },
		"storage": {
			"filesystems": [{
				"device": "$DEVICE",
				"format": "none",
				"path": "/tmp0",
				"wipeFilesystem": false
			}]
		}
	}`
	configMinVersion := "3.3.0"

	in[0].Partitions.GetPartition("EFI-SYSTEM").FilesystemType = "ext4"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		MntDevices:       mntDevices,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}
