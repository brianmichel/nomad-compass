package nomadclient

import (
	"context"
	"errors"

	"github.com/hashicorp/nomad/api"
)

func (a *API) ApplyVariable(_ context.Context, variable *api.Variable) (*api.Variable, error) {
	if variable == nil || variable.Path == "" {
		return nil, errors.New("variable and path are required")
	}
	if variable.ModifyIndex > 0 {
		result, _, err := a.client.Variables().CheckedUpdate(variable, nil)
		return result, err
	}
	result, _, err := a.client.Variables().Update(variable, nil)
	return result, err
}

func (a *API) CreateVariable(_ context.Context, variable *api.Variable) (*api.Variable, error) {
	if variable == nil || variable.Path == "" {
		return nil, errors.New("variable and path are required")
	}
	result, _, err := a.client.Variables().CheckedCreate(variable, nil)
	return result, err
}

func (a *API) ObserveVariable(_ context.Context, namespace, path string) (*api.Variable, error) {
	if path == "" {
		return nil, nil
	}
	variable, meta, err := a.client.Variables().Read(path, &api.QueryOptions{Namespace: namespace})
	if variable == nil && meta != nil && meta.LastIndex == 1 && meta.KnownLeader {
		return nil, errors.New("permission denied reading variable")
	}
	if isNotFound(err) || errors.Is(err, api.ErrVariablePathNotFound) {
		return nil, nil
	}
	return variable, err
}

func (a *API) DeleteVariable(_ context.Context, namespace, path string) error {
	if path == "" {
		return nil
	}
	_, err := a.client.Variables().Delete(path, &api.WriteOptions{Namespace: namespace})
	if isNotFound(err) || errors.Is(err, api.ErrVariablePathNotFound) {
		return nil
	}
	return err
}
