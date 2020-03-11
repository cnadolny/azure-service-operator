// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azuresqlaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-service-operator/api/v1alpha1"
	"github.com/Azure/azure-service-operator/pkg/resourcemanager"
	"github.com/Azure/azure-service-operator/pkg/secrets"
	keyvaultsecretlib "github.com/Azure/azure-service-operator/pkg/secrets/keyvault"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const usernameLength = 8
const passwordLength = 16

// Ensure creates an AzureSqlAction
func (s *AzureSqlActionManager) Ensure(ctx context.Context, obj runtime.Object, opts ...resourcemanager.ConfigOption) (bool, error) {
	instance, err := s.convert(obj)
	if err != nil {
		return false, err
	}
	var key types.NamespacedName
	var adminSecretClient secrets.SecretClient
	serverName := instance.Spec.ServerName
	groupName := instance.Spec.ResourceGroup

	if strings.ToLower(instance.Spec.ActionName) == "rolladmincreds" {

		if !instance.Status.Provisioned {

			// Determine the secret key based on the spec
			if len(instance.Spec.ServerAdminSecretName) == 0 {
				key = types.NamespacedName{Name: instance.Spec.ServerName, Namespace: instance.Namespace}
			} else {
				key = types.NamespacedName{Name: instance.Spec.ServerAdminSecretName}
			}

			// Determine secretclient based on Spec. If Keyvault name isn't specified, fall back to
			// global secret client
			if len(instance.Spec.ServerSecretKeyVault) == 0 {
				adminSecretClient = s.SecretClient
			} else {
				adminSecretClient = keyvaultsecretlib.New(instance.Spec.ServerSecretKeyVault)
				if !keyvaultsecretlib.IsKeyVaultAccessible(adminSecretClient) {
					s.Telemetry.LogInfo("requeuing", "Keyvault specified not accessible yet")
					instance.Status.Message = "InvalidKeyVaultAccess: Keyvault not accessible yet"
					return false, nil
				}
			}

			// Roll SQL server's admin password
			err := s.UpdateAdminPassword(ctx, groupName, serverName, key, adminSecretClient)
			if err != nil {
				instance.Status.Message = err.Error()
				return true, nil
			}

			instance.Status.Provisioned = true
			instance.Status.Provisioning = false
			instance.Status.Message = resourcemanager.SuccessMsg
		}

	} else {
		instance.Status.Message = "Unrecognized action"
	}

	return true, nil

}

// Delete removes an AzureSqlAction
func (s *AzureSqlActionManager) Delete(ctx context.Context, obj runtime.Object, opts ...resourcemanager.ConfigOption) (bool, error) {
	return false, nil
}

// GetParents returns the parents of AzureSqlAction
func (s *AzureSqlActionManager) GetParents(obj runtime.Object) ([]resourcemanager.KubeParent, error) {
	instance, err := s.convert(obj)
	if err != nil {
		return nil, err
	}

	serverKey := types.NamespacedName{Name: instance.Spec.ServerName, Namespace: instance.Namespace}

	return []resourcemanager.KubeParent{
		{Key: serverKey, Target: &v1alpha1.AzureSqlServer{}},
	}, nil
}

// GetStatus gets the Status object of the AzureSqlAction instance
func (g *AzureSqlActionManager) GetStatus(obj runtime.Object) (*v1alpha1.ASOStatus, error) {
	instance, err := g.convert(obj)
	if err != nil {
		return nil, err
	}
	return &instance.Status, nil
}

func (s *AzureSqlActionManager) convert(obj runtime.Object) (*v1alpha1.AzureSqlAction, error) {
	local, ok := obj.(*v1alpha1.AzureSqlAction)
	if !ok {
		return nil, fmt.Errorf("failed type assertion on kind: %s", obj.GetObjectKind().GroupVersionKind().String())
	}
	return local, nil
}
