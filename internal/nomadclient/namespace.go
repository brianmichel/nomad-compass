package nomadclient

import (
	"context"
	"errors"

	"github.com/hashicorp/nomad/api"
)

func (a *API) ApplyNamespace(_ context.Context, namespace *api.Namespace) error {
	if namespace == nil || namespace.Name == "" {
		return errors.New("namespace and name are required")
	}
	_, err := a.client.Namespaces().Register(namespace, nil)
	return err
}

func (a *API) ObserveNamespace(_ context.Context, name string) (*api.Namespace, error) {
	if name == "" {
		return nil, nil
	}
	namespace, _, err := a.client.Namespaces().Info(name, nil)
	if isNotFound(err) {
		return nil, nil
	}
	return namespace, err
}

func (a *API) DeleteNamespace(_ context.Context, name string) error {
	if name == "" {
		return nil
	}
	_, err := a.client.Namespaces().Delete(name, nil)
	if isNotFound(err) {
		return nil
	}
	return err
}
