// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Vault Service Key Management API
//
// API for managing and performing operations with keys and vaults. (For the API for managing secrets, see the Vault Service
// Secret Management API. For the API for retrieving secrets, see the Vault Service Secret Retrieval API.)
//

package keymanagement

import (
	"github.com/oracle/oci-go-sdk/v48/common"
)

// VaultReplicaDetails Vault replica details
type VaultReplicaDetails struct {

	// ReplicationId associated with a vault operation
	ReplicationId *string `mandatory:"false" json:"replicationId"`
}

func (m VaultReplicaDetails) String() string {
	return common.PointerString(m)
}
