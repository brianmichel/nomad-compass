package nomadclient

import (
	"context"
	"errors"

	"github.com/hashicorp/nomad/api"
)

func (a *API) ApplySentinelPolicy(_ context.Context, policy *api.SentinelPolicy) error {
	if policy == nil || policy.Name == "" {
		return errors.New("sentinel policy and name are required")
	}
	_, err := a.client.SentinelPolicies().Upsert(policy, nil)
	return err
}

func (a *API) ObserveSentinelPolicy(_ context.Context, name string) (*api.SentinelPolicy, error) {
	if name == "" {
		return nil, nil
	}
	policy, _, err := a.client.SentinelPolicies().Info(name, nil)
	if isNotFound(err) {
		return nil, nil
	}
	return policy, err
}

func (a *API) DeleteSentinelPolicy(_ context.Context, name string) error {
	if name == "" {
		return nil
	}
	_, err := a.client.SentinelPolicies().Delete(name, nil)
	if isNotFound(err) {
		return nil
	}
	return err
}
