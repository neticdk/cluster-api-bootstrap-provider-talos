// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:staticcheck
package v1alpha3

import capiv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

// Conditions and condition Reasons for the TalosConfig object

const (
	// DataSecretAvailableCondition documents the status of the bootstrap secret generation process.
	//
	// NOTE: When the DataSecret generation starts the process completes immediately and within the
	// same reconciliation, so the user will always see a transition from Wait to Generated without having
	// the chance to observe "Waiting" (with the exception of errors or temporary blocks).
	//
	// Deprecated: this condition will be removed when support for CAPI v1beta1 contract will be dropped.
	//
	// +optional
	DataSecretAvailableCondition capiv1.ConditionType = "DataSecretAvailable"

	// DataSecretNotAvailableReason is set when the bootstrap secret was not yet generated.
	//
	// Deprecated: this reason will be removed when support for CAPI v1beta1 contract will be dropped.
	//
	// +optional
	DataSecretNotAvailableReason = "DataSecretNotAvailable"

	// DataSecretNotAvailableInternalErrorReason is set when the bootstrap secret generation failed.
	//
	// Deprecated: this reason will be removed when support for CAPI v1beta1 contract will be dropped.
	//
	// +optional
	DataSecretNotAvailableInternalErrorReason = "DataSecretNotAvailableInternalError"

	// DataSecretAvailableReason is set when the bootstrap secret is successfully generated and ready.
	//
	// Deprecated: this reason will be removed when support for CAPI v1beta1 contract will be dropped.
	//
	// +optional
	DataSecretAvailableReason = "DataSecretAvailable"
)
