// Package privacy implements the PRD 7.4 GDPR data rights: the organization
// data export (right to access and portability), available to tenant admins
// holding the data.export permission, and the full tenant data purge (right
// to erasure), restricted to platform staff and guarded by a slug
// confirmation.
package privacy

import "errors"

// ErrConfirmationMismatch indicates the purge confirmation did not equal the
// organization's slug.
var ErrConfirmationMismatch = errors.New("confirmation does not match the organization slug")
