package employees

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// AccountCreator creates login accounts for employees.
type AccountCreator interface {
	CreateUserAccount(ctx context.Context, email, displayName, password string) (userID string, err error)
}

// AccountResolver resolves an existing account by email. It is an optional
// extension of AccountCreator that lets provisioning reuse the account a
// previous failed attempt created instead of bricking the retry on
// EMAIL_TAKEN.
type AccountResolver interface {
	FindUserIDByEmail(ctx context.Context, email string) (string, error)
}

// MemberAdder adds organization memberships.
type MemberAdder interface {
	AddEmployeeMember(ctx context.Context, organizationID, userID string) error
}

var (
	errProvisionAccount = errors.New("provision account failed")
	errProvisionMember  = errors.New("provision member failed")
)

// Provisioner orchestrates access provisioning for an invited employee:
// account creation, org membership, and the employee link.
type Provisioner struct {
	svc      *Service
	accounts AccountCreator
	members  MemberAdder
}

// NewProvisioner constructs a Provisioner.
func NewProvisioner(svc *Service, accounts AccountCreator, members MemberAdder) *Provisioner {
	return &Provisioner{svc: svc, accounts: accounts, members: members}
}

// Provision grants an invited employee portal access and returns the updated
// employee and the provisioned user ID.
func (p *Provisioner) Provision(
	ctx context.Context,
	organizationID, employeeID, displayName, password string,
) (Employee, string, error) {
	employee, err := p.svc.Get(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, "", err
	}

	if employee.UserID != "" {
		return Employee{}, "", ErrAlreadyProvisioned
	}

	if displayName == "" {
		displayName = employee.FirstName + " " + employee.LastName
	}

	userID, err := p.accounts.CreateUserAccount(ctx, employee.WorkEmail, displayName, password)
	if err != nil {
		userID, err = p.reuseExistingAccount(ctx, employee.WorkEmail, err)
		if err != nil {
			return Employee{}, "", err
		}
	}

	if err := p.members.AddEmployeeMember(ctx, organizationID, userID); err != nil {
		return Employee{}, "", fmt.Errorf("%w: %w", errProvisionMember, err)
	}

	updated, err := p.svc.LinkUser(ctx, organizationID, employeeID, userID)
	if err != nil {
		return Employee{}, "", err
	}

	return updated, userID, nil
}

// reuseExistingAccount resolves the account a previous failed provisioning
// attempt left behind, so a retry is not bricked by EMAIL_TAKEN.
func (p *Provisioner) reuseExistingAccount(ctx context.Context, email string, createErr error) (string, error) {
	resolver, ok := p.accounts.(AccountResolver)
	if !ok {
		return "", fmt.Errorf("%w: %w", errProvisionAccount, createErr)
	}

	userID, err := resolver.FindUserIDByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errProvisionAccount, createErr)
	}

	slog.WarnContext(ctx, "reusing existing account for employee provisioning", "userId", userID)

	return userID, nil
}
