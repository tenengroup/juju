// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package credentialcommon

import (
	"github.com/juju/collections/set"
	"github.com/juju/errors"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/cloud"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/context"
)

// ValidateModelCredential checks if a cloud credential is valid for a model.
func ValidateModelCredential(persisted CloudEntitiesBackend, provider CloudProvider, callCtx context.ProviderCallContext) (params.ErrorResults, error) {
	// We only check persisted machines vs known cloud instances.
	// In the future, this check may be extended to other cloud resources,
	// entities and operation-level authorisations such as interfaces,
	// ability to CRUD storage, etc.
	return CheckMachineInstances(persisted, provider, callCtx)
}

// ValidateNewModelCredential checks if a new cloud credential can be valid
// for a given model.
// Note that this call does not validate credential against the cloud of the model.
func ValidateNewModelCredential(backend ModelBackend, newEnv NewEnvironFunc, callCtx context.ProviderCallContext, credential *cloud.Credential) (params.ErrorResults, error) {
	fail := func(original error) (params.ErrorResults, error) {
		return params.ErrorResults{}, original
	}
	model, err := backend.Model()
	if err != nil {
		return fail(errors.Trace(err))
	}

	modelCloud, err := backend.Cloud(model.Cloud())
	if err != nil {
		return fail(errors.Trace(err))
	}
	tempCloudSpec, err := environs.MakeCloudSpec(modelCloud, model.CloudRegion(), credential)
	if err != nil {
		return fail(errors.Trace(err))
	}

	cfg, err := model.Config()
	if err != nil {
		return fail(errors.Trace(err))
	}
	tempOpenParams := environs.OpenParams{
		Cloud:  tempCloudSpec,
		Config: cfg,
	}
	env, err := newEnv(tempOpenParams)
	if err != nil {
		return fail(errors.Trace(err))
	}

	return ValidateModelCredential(backend, env, callCtx)
}

// CheckMachineInstances compares model machines from state with
// the ones reported by the provider using supplied credential.
func CheckMachineInstances(backend CloudEntitiesBackend, provider CloudProvider, callCtx context.ProviderCallContext) (params.ErrorResults, error) {
	fail := func(original error) (params.ErrorResults, error) {
		return params.ErrorResults{}, original
	}

	// Get machines from state
	machines, err := backend.AllMachines()
	if err != nil {
		return fail(errors.Trace(err))
	}

	var results []params.ErrorResult

	serverError := func(received error) params.ErrorResult {
		return params.ErrorResult{Error: common.ServerError(received)}
	}

	machinesByInstance := make(map[string]string)
	for _, machine := range machines {
		if machine.IsContainer() {
			// Containers don't correspond to instances at the
			// provider level.
			continue
		}
		if manual, err := machine.IsManual(); err != nil {
			return fail(errors.Trace(err))
		} else if manual {
			continue
		}
		instanceId, err := machine.InstanceId()
		if err != nil {
			results = append(results, serverError(errors.Annotatef(err, "getting instance id for machine %s", machine.Id())))
			continue
		}
		machinesByInstance[string(instanceId)] = machine.Id()
	}

	// Check can see all machines' instances
	instances, err := provider.AllInstances(callCtx)
	if err != nil {
		return fail(errors.Trace(err))
	}

	instanceIds := set.NewStrings()
	for _, instance := range instances {
		id := string(instance.Id())
		instanceIds.Add(id)
		if _, found := machinesByInstance[id]; !found {
			results = append(results, serverError(errors.Errorf("no machine with instance %q", id)))
		}
	}

	for instanceId, name := range machinesByInstance {
		if !instanceIds.Contains(instanceId) {
			results = append(results, serverError(errors.Errorf("couldn't find instance %q for machine %s", instanceId, name)))
		}
	}

	return params.ErrorResults{Results: results}, nil
}

// NewEnvironFunc defines function that obtains new Environ.
type NewEnvironFunc func(args environs.OpenParams) (environs.Environ, error)
