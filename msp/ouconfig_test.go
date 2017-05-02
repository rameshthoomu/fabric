/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package msp

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBadConfigOU(t *testing.T) {
	// testdata/badconfigou:
	// the configuration is such that only identities
	// with OU=COP2 and signed by the root ca should be validated
	conf, err := GetLocalMspConfig("testdata/badconfigou", nil, "DEFAULT")
	if err != nil {
		fmt.Printf("Setup should have succeeded, got err %s instead", err)
		os.Exit(-1)
	}

	thisMSP, err := NewBccspMsp()
	assert.NoError(t, err)

	err = thisMSP.Setup(conf)
	assert.NoError(t, err)

	id, err := thisMSP.GetDefaultSigningIdentity()
	assert.NoError(t, err)

	// the default signing identity OU is COP but the msp is configured
	// to validate only identities whose OU is COP2
	err = id.Validate()
	assert.Error(t, err)
}

func TestBadConfigOUCert(t *testing.T) {
	// testdata/badconfigoucert:
	// the configuration of the OU identifier points to a
	// certificate that is neither a CA nor an intermediate CA for the msp.
	conf, err := GetLocalMspConfig("testdata/badconfigoucert", nil, "DEFAULT")
	assert.NoError(t, err)

	thisMSP, err := NewBccspMsp()
	assert.NoError(t, err)

	err = thisMSP.Setup(conf)
	assert.Error(t, err)
}
