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
	result, _, err := a.client.Variables().Update(variable, nil)
	return result, err
}

func (a *API) ObserveVariable(_ context.Context, namespace, path string) (*api.Variable, error) {
	if path == "" {
		return nil, nil
	}
	variable, _, err := a.client.Variables().Peek(path, &api.QueryOptions{Namespace: namespace})
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
