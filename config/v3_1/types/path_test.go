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

package types

import (
	"reflect"
	"testing"

	"github.com/flatcar-linux/ignition/v2/config/shared/errors"
)

func TestPathValidate(t *testing.T) {
	tests := []struct {
		in  string
		out error
	}{
		{
			"/good/path",
			nil,
		},
		{
			"/name",
			nil,
		},
		{
			"/this/is/a/fairly/long/path/to/a/device.",
			nil,
		},
		{
			"/this one has spaces",
			nil,
		},
		{
			"relative/path",
			errors.ErrPathRelative,
		},
	}

	for i, test := range tests {
		err := validatePath(test.in)
		if !reflect.DeepEqual(test.out, err) {
			t.Errorf("#%d: bad error: want %v, got %v", i, test.out, err)
		}
	}
}
