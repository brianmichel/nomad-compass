package nomadclient

import (
	"context"
	"errors"

	"github.com/hashicorp/nomad/api"
)

func (a *API) ApplyQuota(_ context.Context, quota *api.QuotaSpec) error {
	if quota == nil || quota.Name == "" {
		return errors.New("quota and name are required")
	}
	_, err := a.client.Quotas().Register(quota, nil)
	return err
}

func (a *API) ObserveQuota(_ context.Context, name string) (*api.QuotaSpec, error) {
	if name == "" {
		return nil, nil
	}
	quota, _, err := a.client.Quotas().Info(name, nil)
	if isNotFound(err) {
		return nil, nil
	}
	return quota, err
}

func (a *API) DeleteQuota(_ context.Context, name string) error {
	if name == "" {
		return nil
	}
	_, err := a.client.Quotas().Delete(name, nil)
	if isNotFound(err) {
		return nil
	}
	return err
}
