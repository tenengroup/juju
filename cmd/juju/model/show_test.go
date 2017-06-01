// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for info.

package model_test

import (
	"time"

	"github.com/juju/cmd"
	"github.com/juju/cmd/cmdtesting"
	"github.com/juju/errors"
	gitjujutesting "github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/version"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/cmd/juju/model"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/juju/status"
	"github.com/juju/juju/testing"
)

var _ = gc.Suite(&ShowCommandSuite{})
var _ = gc.Suite(&showSLACommandSuite{})

type ShowCommandSuite struct {
	testing.FakeJujuXDGDataHomeSuite
	fake            fakeModelShowClient
	store           *jujuclient.MemStore
	expectedOutput  attrs
	expectedDisplay string
}

func (s *ShowCommandSuite) SetUpTest(c *gc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)
	lastConnection := time.Date(2015, 3, 20, 0, 0, 0, 0, time.UTC)
	statusSince := time.Date(2016, 4, 5, 0, 0, 0, 0, time.UTC)
	migrationStart := time.Date(2016, 4, 6, 0, 10, 0, 0, time.UTC)
	migrationEnd := time.Date(2016, 4, 7, 0, 0, 15, 0, time.UTC)

	users := []params.ModelUserInfo{{
		UserName:       "admin",
		LastConnection: &lastConnection,
		Access:         "write",
	}, {
		UserName:    "bob",
		DisplayName: "Bob",
		Access:      "read",
	}}

	s.fake = fakeModelShowClient{
		info: params.ModelInfo{
			Name:           "mymodel",
			UUID:           testing.ModelTag.Id(),
			ControllerUUID: "1ca2293b-fdb9-4299-97d6-55583bb39364",
			OwnerTag:       "user-admin",
			CloudTag:       "cloud-some-cloud",
			CloudRegion:    "some-region",
			ProviderType:   "openstack",
			Life:           params.Alive,
			Status: params.EntityStatus{
				Status: status.Active,
				Since:  &statusSince,
			},
			Users: users,
			Migration: &params.ModelMigrationStatus{
				Status: "obfuscating Quigley matrix",
				Start:  &migrationStart,
				End:    &migrationEnd,
			},
		},
	}

	s.expectedOutput = attrs{
		"mymodel": attrs{
			"name":            "mymodel",
			"model-uuid":      "deadbeef-0bad-400d-8000-4b1d0d06f00d",
			"controller-uuid": "1ca2293b-fdb9-4299-97d6-55583bb39364",
			"controller-name": "testing",
			"owner":           "admin",
			"cloud":           "some-cloud",
			"region":          "some-region",
			"type":            "openstack",
			"life":            "alive",
			"status": attrs{
				"current":         "active",
				"since":           "2016-04-05",
				"migration":       "obfuscating Quigley matrix",
				"migration-start": "2016-04-06",
				"migration-end":   "2016-04-07",
			},
			"users": attrs{
				"admin": attrs{
					"access":          "write",
					"last-connection": "2015-03-20",
				},
				"bob": attrs{
					"display-name":    "Bob",
					"access":          "read",
					"last-connection": "never connected",
				},
			},
		},
	}

	s.store = jujuclient.NewMemStore()
	s.store.CurrentControllerName = "testing"
	s.store.Controllers["testing"] = jujuclient.ControllerDetails{}
	s.store.Accounts["testing"] = jujuclient.AccountDetails{
		User: "admin",
	}
	err := s.store.UpdateModel("testing", "admin/mymodel", jujuclient.ModelDetails{
		testing.ModelTag.Id(),
	})
	c.Assert(err, jc.ErrorIsNil)
	s.store.Models["testing"].CurrentModel = "admin/mymodel"
}

func (s *ShowCommandSuite) TestShow(c *gc.C) {
	_, err := cmdtesting.RunCommand(c, s.newShowCommand())
	c.Assert(err, jc.ErrorIsNil)
	s.fake.CheckCalls(c, []gitjujutesting.StubCall{
		{"ModelInfo", []interface{}{[]names.ModelTag{testing.ModelTag}}},
		{"Close", nil},
	})
}

func (s *ShowCommandSuite) TestShowUnknownCallsRefresh(c *gc.C) {
	called := false
	refresh := func(jujuclient.ClientStore, string) error {
		called = true
		return nil
	}
	_, err := cmdtesting.RunCommand(c, model.NewShowCommandForTest(&s.fake, refresh, s.store), "unknown")
	c.Check(called, jc.IsTrue)
	c.Check(err, jc.Satisfies, errors.IsNotFound)
}

func (s *ShowCommandSuite) TestShowFormatYaml(c *gc.C) {
	ctx, err := cmdtesting.RunCommand(c, s.newShowCommand(), "--format", "yaml")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(ctx), jc.YAMLEquals, s.expectedOutput)
}

func (s *ShowCommandSuite) TestShowFormatJson(c *gc.C) {
	ctx, err := cmdtesting.RunCommand(c, s.newShowCommand(), "--format", "json")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(ctx), jc.JSONEquals, s.expectedOutput)
}

func (s *ShowCommandSuite) TestUnrecognizedArg(c *gc.C) {
	_, err := cmdtesting.RunCommand(c, s.newShowCommand(), "admin", "whoops")
	c.Assert(err, gc.ErrorMatches, `unrecognized args: \["whoops"\]`)
}

func (s *ShowCommandSuite) TestShowBasicIncompleteModelsYaml(c *gc.C) {
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: createBasicModelInfo()},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicIncompleteModelsJson(c *gc.C) {
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: createBasicModelInfo()},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\"}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithStatusIncompleteModelsYaml(c *gc.C) {
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: createBasicModelInfoWithStatus()},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  status:
    current: busy
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithStatusIncompleteModelsJson(c *gc.C) {
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: createBasicModelInfoWithStatus()},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\",\"status\":{\"current\":\"busy\"}}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithMigrationIncompleteModelsYaml(c *gc.C) {
	basicAndMigrationStatusInfo := createBasicModelInfo()
	addMigrationStatusStatus(basicAndMigrationStatusInfo)
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndMigrationStatusInfo},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  status:
    migration: importing
    migration-start: just now
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithMigrationIncompleteModelsJson(c *gc.C) {
	basicAndMigrationStatusInfo := createBasicModelInfo()
	addMigrationStatusStatus(basicAndMigrationStatusInfo)
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndMigrationStatusInfo},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\"," +
		"\"status\":{\"migration\":\"importing\"," +
		"\"migration-start\":\"just now\"}}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithStatusAndMigrationIncompleteModelsYaml(c *gc.C) {
	basicAndStatusAndMigrationInfo := createBasicModelInfoWithStatus()
	addMigrationStatusStatus(basicAndStatusAndMigrationInfo)
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndStatusAndMigrationInfo},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  status:
    current: busy
    migration: importing
    migration-start: just now
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithStatusAndMigrationIncompleteModelsJson(c *gc.C) {
	basicAndStatusAndMigrationInfo := createBasicModelInfoWithStatus()
	addMigrationStatusStatus(basicAndStatusAndMigrationInfo)
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndStatusAndMigrationInfo},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\",\"status\":{\"current\":\"busy\"," +
		"\"migration\":\"importing\",\"migration-start\":\"just now\"}}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithProviderIncompleteModelsYaml(c *gc.C) {
	basicAndProviderTypeInfo := createBasicModelInfo()
	basicAndProviderTypeInfo.ProviderType = "aws"
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndProviderTypeInfo},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  type: aws
  life: dead
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithProviderIncompleteModelsJson(c *gc.C) {
	basicAndProviderTypeInfo := createBasicModelInfo()
	basicAndProviderTypeInfo.ProviderType = "aws"
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndProviderTypeInfo},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"type\":\"aws\",\"life\":\"dead\"}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithUsersIncompleteModelsYaml(c *gc.C) {
	basicAndUsersInfo := createBasicModelInfo()
	basicAndUsersInfo.Users = []params.ModelUserInfo{
		params.ModelUserInfo{"admin", "display name", nil, params.UserAccessPermission("admin")},
	}
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndUsersInfo},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  users:
    admin:
      display-name: display name
      access: admin
      last-connection: never connected
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithUsersIncompleteModelsJson(c *gc.C) {
	basicAndUsersInfo := createBasicModelInfo()
	basicAndUsersInfo.Users = []params.ModelUserInfo{
		params.ModelUserInfo{"admin", "display name", nil, params.UserAccessPermission("admin")},
	}

	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndUsersInfo},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\"," +
		"\"users\":{\"admin\":{\"display-name\":\"display name\"," +
		"\"access\":\"admin\",\"last-connection\":\"never connected\"}}}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithMachinesIncompleteModelsYaml(c *gc.C) {
	basicAndMachinesInfo := createBasicModelInfo()
	basicAndMachinesInfo.Machines = []params.ModelMachineInfo{
		params.ModelMachineInfo{Id: "2"},
		params.ModelMachineInfo{Id: "12"},
	}
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndMachinesInfo},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  machines:
    "2":
      cores: 0
    "12":
      cores: 0
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithMachinesIncompleteModelsJson(c *gc.C) {
	basicAndMachinesInfo := createBasicModelInfo()
	basicAndMachinesInfo.Machines = []params.ModelMachineInfo{
		params.ModelMachineInfo{Id: "2"},
		params.ModelMachineInfo{Id: "12"},
	}
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndMachinesInfo},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\"," +
		"\"machines\":{\"12\":{\"cores\":0},\"2\":{\"cores\":0}}}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowBasicWithSLAIncompleteModelsYaml(c *gc.C) {
	basicAndSLAInfo := createBasicModelInfo()
	basicAndSLAInfo.SLA = &params.ModelSLAInfo{
		Owner: "owner",
		Level: "level",
	}
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndSLAInfo},
	}
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  sla: level
  sla-owner: owner
`[1:]
	s.assertShowOutput(c, "yaml")
}

func (s *ShowCommandSuite) TestShowBasicWithSLAIncompleteModelsJson(c *gc.C) {
	basicAndSLAInfo := createBasicModelInfo()
	basicAndSLAInfo.SLA = &params.ModelSLAInfo{
		Owner: "owner",
		Level: "level",
	}
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicAndSLAInfo},
	}
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\",\"sla\":\"level\"," +
		"\"sla-owner\":\"owner\"}}\n"
	s.assertShowOutput(c, "json")
}

func (s *ShowCommandSuite) TestShowModelWithAgentVersionInJson(c *gc.C) {
	s.expectedDisplay = "{\"basic-model\":{\"name\":\"basic-model\"," +
		"\"model-uuid\":\"deadbeef-0bad-400d-8000-4b1d0d06f00d\"," +
		"\"controller-uuid\":\"deadbeef-1bad-500d-9000-4b1d0d06f00d\"," +
		"\"controller-name\":\"testing\",\"owner\":\"owner\"," +
		"\"cloud\":\"altostratus\",\"region\":\"mid-level\"," +
		"\"life\":\"dead\",\"agent-version\":\"2.55.5\"}}\n"
	s.assertShowModelWithAgent(c, "json")
}

func (s *ShowCommandSuite) TestShowModelWithAgentVersionInYaml(c *gc.C) {
	s.expectedDisplay = `
basic-model:
  name: basic-model
  model-uuid: deadbeef-0bad-400d-8000-4b1d0d06f00d
  controller-uuid: deadbeef-1bad-500d-9000-4b1d0d06f00d
  controller-name: testing
  owner: owner
  cloud: altostratus
  region: mid-level
  life: dead
  agent-version: 2.55.5
`[1:]
	s.assertShowModelWithAgent(c, "yaml")
}

func (s *ShowCommandSuite) assertShowModelWithAgent(c *gc.C, format string) {
	// Since most of the tests in this suite already test model infos without
	// agent version, all we need to do here is to test one with it.
	agentVersion, err := version.Parse("2.55.5")
	c.Assert(err, jc.ErrorIsNil)
	basicTestInfo := createBasicModelInfo()
	basicTestInfo.AgentVersion = &agentVersion
	s.fake.infos = []params.ModelInfoResult{
		params.ModelInfoResult{Result: basicTestInfo},
	}
	s.assertShowOutput(c, format)
}

func (s *ShowCommandSuite) newShowCommand() cmd.Command {
	return model.NewShowCommandForTest(&s.fake, noOpRefresh, s.store)
}

func (s *ShowCommandSuite) assertShowOutput(c *gc.C, format string) {
	ctx, err := cmdtesting.RunCommand(c, s.newShowCommand(), "--format", format)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(ctx), gc.Equals, s.expectedDisplay)
}

func createBasicModelInfo() *params.ModelInfo {
	return &params.ModelInfo{
		Name:           "basic-model",
		UUID:           testing.ModelTag.Id(),
		ControllerUUID: testing.ControllerTag.Id(),
		OwnerTag:       names.NewUserTag("owner").String(),
		Life:           params.Dead,
		CloudTag:       names.NewCloudTag("altostratus").String(),
		CloudRegion:    "mid-level",
	}
}

func createBasicModelInfoWithStatus() *params.ModelInfo {
	basicAndStatusInfo := createBasicModelInfo()
	basicAndStatusInfo.Status = params.EntityStatus{
		Status: status.Busy,
	}
	return basicAndStatusInfo
}

func addMigrationStatusStatus(existingInfo *params.ModelInfo) {
	now := time.Now()
	existingInfo.Migration = &params.ModelMigrationStatus{
		Status: "importing",
		Start:  &now,
	}
}

type showSLACommandSuite struct {
	ShowCommandSuite
}

func (s *showSLACommandSuite) SetUpTest(c *gc.C) {
	s.ShowCommandSuite.SetUpTest(c)

	s.fake.info.SLA = &params.ModelSLAInfo{
		Level: "next",
		Owner: "user",
	}
	slaOutput := s.expectedOutput["mymodel"].(attrs)
	slaOutput["sla"] = "next"
	slaOutput["sla-owner"] = "user"
}

func noOpRefresh(jujuclient.ClientStore, string) error {
	return nil
}

type attrs map[string]interface{}

type fakeModelShowClient struct {
	gitjujutesting.Stub
	info  params.ModelInfo
	err   *params.Error
	infos []params.ModelInfoResult
}

func (f *fakeModelShowClient) Close() error {
	f.MethodCall(f, "Close")
	return f.NextErr()
}

func (f *fakeModelShowClient) ModelInfo(tags []names.ModelTag) ([]params.ModelInfoResult, error) {
	f.MethodCall(f, "ModelInfo", tags)
	if f.infos != nil {
		return f.infos, nil
	}
	if len(tags) != 1 {
		return nil, errors.Errorf("expected 1 tag, got %d", len(tags))
	}
	if tags[0] != testing.ModelTag {
		return nil, errors.Errorf("expected %s, got %s", testing.ModelTag.Id(), tags[0].Id())
	}
	return []params.ModelInfoResult{{Result: &f.info, Error: f.err}}, f.NextErr()
}
