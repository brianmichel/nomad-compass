package nomadclient

import (
	"context"
	"errors"
	"net/http"

	"github.com/hashicorp/nomad/api"
)

// ResourceClient contains Nomad operations for bundle-managed resources. It
// is separate from Client so existing job-only fakes remain source-compatible.
type ResourceClient interface {
	Client
	ApplyHostVolume(ctx context.Context, volume *api.HostVolume) (*api.HostVolume, error)
	ObserveHostVolume(ctx context.Context, id, namespace string) (*api.HostVolume, error)
	DeleteHostVolume(ctx context.Context, id, namespace string, force bool) error
	ApplyCSIVolume(ctx context.Context, volume *api.CSIVolume) (*api.CSIVolume, error)
	ObserveCSIVolume(ctx context.Context, id, namespace string) (*api.CSIVolume, error)
	DeleteCSIVolume(ctx context.Context, id, namespace string, force bool) error
	ApplyACLPolicy(ctx context.Context, policy *api.ACLPolicy) error
	ObserveACLPolicy(ctx context.Context, name string) (*api.ACLPolicy, error)
	DeleteACLPolicy(ctx context.Context, name string) error
}

func (a *API) ApplyHostVolume(_ context.Context, volume *api.HostVolume) (*api.HostVolume, error) {
	if volume == nil {
		return nil, errors.New("host volume is required")
	}

	if volume.ID == "" {
		stubs, _, err := a.client.HostVolumes().List(&api.HostVolumeListRequest{}, &api.QueryOptions{Namespace: volume.Namespace})
		if err != nil {
			return nil, err
		}
		for _, stub := range stubs {
			if stub != nil && stub.Name == volume.Name && effectiveNamespace(stub.Namespace) == effectiveNamespace(volume.Namespace) {
				volume.ID = stub.ID
				break
			}
		}
	}

	resp, _, err := a.client.HostVolumes().Create(&api.HostVolumeCreateRequest{Volume: volume}, &api.WriteOptions{Namespace: volume.Namespace})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Volume == nil {
		return nil, errors.New("Nomad returned an empty host volume response")
	}
	return resp.Volume, nil
}

func (a *API) ObserveHostVolume(_ context.Context, id, namespace string) (*api.HostVolume, error) {
	if id == "" {
		return nil, nil
	}
	volume, _, err := a.client.HostVolumes().Get(id, &api.QueryOptions{Namespace: namespace})
	if isNotFound(err) {
		return nil, nil
	}
	return volume, err
}

func (a *API) DeleteHostVolume(_ context.Context, id, namespace string, force bool) error {
	if id == "" {
		return nil
	}
	_, _, err := a.client.HostVolumes().Delete(&api.HostVolumeDeleteRequest{ID: id, Force: force}, &api.WriteOptions{Namespace: namespace})
	if isNotFound(err) {
		return nil
	}
	return err
}

func (a *API) ApplyCSIVolume(_ context.Context, volume *api.CSIVolume) (*api.CSIVolume, error) {
	if volume == nil || volume.ID == "" {
		return nil, errors.New("CSI volume and ID are required")
	}
	write := &api.CSIVolumeRegisterRequest{Volumes: []*api.CSIVolume{volume}}
	if volume.ExternalID != "" {
		resp, _, err := a.client.CSIVolumes().RegisterOpts(write, &api.WriteOptions{Namespace: volume.Namespace})
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.Volumes) == 0 {
			return nil, errors.New("Nomad returned an empty CSI volume response")
		}
		return resp.Volumes[0], nil
	}

	resp, _, err := a.client.CSIVolumes().CreateOpts(&api.CSIVolumeCreateRequest{
		Volumes: []*api.CSIVolume{volume},
	}, &api.WriteOptions{Namespace: volume.Namespace})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Volumes) == 0 {
		return nil, errors.New("Nomad returned an empty CSI volume response")
	}
	return resp.Volumes[0], nil
}

func (a *API) ObserveCSIVolume(_ context.Context, id, namespace string) (*api.CSIVolume, error) {
	if id == "" {
		return nil, nil
	}
	volume, _, err := a.client.CSIVolumes().Info(id, &api.QueryOptions{Namespace: namespace})
	if isNotFound(err) {
		return nil, nil
	}
	return volume, err
}

func (a *API) DeleteCSIVolume(_ context.Context, id, namespace string, force bool) error {
	if id == "" {
		return nil
	}
	err := a.client.CSIVolumes().Deregister(id, force, &api.WriteOptions{Namespace: namespace})
	if isNotFound(err) {
		return nil
	}
	return err
}

func (a *API) ApplyACLPolicy(_ context.Context, policy *api.ACLPolicy) error {
	if policy == nil || policy.Name == "" {
		return errors.New("ACL policy and name are required")
	}
	_, err := a.client.ACLPolicies().Upsert(policy, nil)
	return err
}

func (a *API) ObserveACLPolicy(_ context.Context, name string) (*api.ACLPolicy, error) {
	if name == "" {
		return nil, nil
	}
	policy, _, err := a.client.ACLPolicies().Info(name, nil)
	if isNotFound(err) {
		return nil, nil
	}
	return policy, err
}

func (a *API) DeleteACLPolicy(_ context.Context, name string) error {
	if name == "" {
		return nil
	}
	_, err := a.client.ACLPolicies().Delete(name, nil)
	if isNotFound(err) {
		return nil
	}
	return err
}

func effectiveNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var unexpected api.UnexpectedResponseError
	return errors.As(err, &unexpected) && unexpected.StatusCode() == http.StatusNotFound
}
