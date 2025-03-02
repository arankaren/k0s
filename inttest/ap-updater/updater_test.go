// Copyright 2021 k0s authors
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

package updater

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/suite"
)

const (
	ManifestTestDirPerms = "775"
)

type plansSingleControllerSuite struct {
	common.FootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *plansSingleControllerSuite) SetupTest() {
	ctx := s.Context()
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))

	s.Require().NoError(s.InitController(0), "--disable-components=metrics-server")
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	client, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "controlnodes"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "updateconfigs"))
}

// TestApply applies a well-formed `plan` yaml, and asserts that all of the correct values
// across different objects are correct.
func (s *plansSingleControllerSuite) TestApply() {
	updaterConfig := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: UpdateConfig
metadata:
  name: autopilot
spec:
  channel: stable
  updateServer: {{.Address}}
  upgradeStrategy:
    cron: "* * * * * *"
  planSpec:
    commands:
    - k0supdate:
        forceupdate: true
        targets:
          controllers:
            discovery:
              selector: {}
`

	vars := struct {
		Address string
	}{
		Address: fmt.Sprintf("http://%s", s.GetUpdateServerIPAddress()),
	}

	manifestFile := "/tmp/updateconfig.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, updaterConfig, vars)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	_, err = aptest.WaitForPlanState(s.Context(), client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)
}

// TestPlansSingleControllerSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlansSingleControllerSuite(t *testing.T) {
	suite.Run(t, &plansSingleControllerSuite{
		common.FootlooseSuite{
			ControllerCount:  1,
			WorkerCount:      0,
			WithUpdateServer: true,
			LaunchMode:       common.LaunchModeOpenRC,
		},
	})
}
