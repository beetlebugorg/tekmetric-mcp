package tekmetric

import (
	"fmt"
	"strings"
)

// normalizeSortDirection checks a sort direction and returns it in upper case.
// An empty direction is valid and stays empty.
func normalizeSortDirection(direction string) (string, error) {
	if direction == "" {
		return "", nil
	}

	upper := strings.ToUpper(direction)
	if upper != "ASC" && upper != "DESC" {
		return "", fmt.Errorf("invalid sort direction '%s': must be ASC or DESC", direction)
	}
	return upper, nil
}

// validateSortFields checks a comma separated list of sort fields.
func validateSortFields(sort string, valid map[string]bool, supported string) error {
	if sort == "" {
		return nil
	}

	for _, field := range strings.Split(sort, ",") {
		trimmed := strings.TrimSpace(field)
		if !valid[trimmed] {
			return fmt.Errorf("invalid sort field '%s': supported fields are %s", trimmed, supported)
		}
	}
	return nil
}

// Validate validates the RepairOrderQueryParams
func (p *RepairOrderQueryParams) Validate() error {
	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	// Validate sort field - based on Tekmetric API documentation
	if p.Sort != "" {
		validSorts := map[string]bool{
			"createdDate":        true,
			"repairOrderNumber":  true,
			"customer.firstName": true,
			"customer.lastName":  true,
		}
		if !validSorts[p.Sort] {
			return fmt.Errorf("invalid sort field '%s': supported fields are createdDate, repairOrderNumber, customer.firstName, customer.lastName", p.Sort)
		}
	}

	// Validate repair order status IDs
	for _, statusID := range p.RepairOrderStatusIds {
		if statusID < 1 || statusID > 7 {
			return fmt.Errorf("invalid repairOrderStatusId '%d': must be 1-7 (1=Estimate, 2=WIP, 3=Complete, 4=Saved, 5=Posted, 6=AR, 7=Deleted)", statusID)
		}
	}

	return nil
}

// Validate validates the CustomerQueryParams
func (p *CustomerQueryParams) Validate() error {
	// Validate customer type ID
	if p.CustomerTypeID != 0 && p.CustomerTypeID != 1 && p.CustomerTypeID != 2 {
		return fmt.Errorf("invalid customerTypeId '%d': must be 1 (Customer) or 2 (Business)", p.CustomerTypeID)
	}

	// Sort accepts a comma separated list.
	validSorts := map[string]bool{"lastName": true, "firstName": true, "email": true}
	if err := validateSortFields(p.Sort, validSorts, "lastName, firstName, email"); err != nil {
		return err
	}

	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	return nil
}

// Validate validates the VehicleQueryParams
func (p *VehicleQueryParams) Validate() error {
	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	// Note: API documentation doesn't specify allowed sort fields for vehicles
	// So we don't validate the Sort field - let the API reject invalid values

	return nil
}

// Validate validates the AppointmentQueryParams
func (p *AppointmentQueryParams) Validate() error {
	// The API returns deleted appointments unless the caller says otherwise.
	if p.IncludeDeleted == nil {
		p.IncludeDeleted = new(bool)
	}

	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	// Note: API documentation doesn't specify allowed sort fields for appointments
	// So we don't validate the Sort field - let the API reject invalid values

	return nil
}

// Validate validates the JobQueryParams
func (p *JobQueryParams) Validate() error {
	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	// Validate sort field - based on Tekmetric API documentation
	if p.Sort != "" && p.Sort != "authorizedDate" {
		return fmt.Errorf("invalid sort field '%s': only 'authorizedDate' is supported", p.Sort)
	}

	// Validate repair order status IDs (jobs don't support status 7 - Deleted)
	for _, statusID := range p.RepairOrderStatusIds {
		if statusID < 1 || statusID > 6 {
			return fmt.Errorf("invalid repairOrderStatusId '%d': must be 1-6 (1=Estimate, 2=WIP, 3=Complete, 4=Saved, 5=Posted, 6=AR)", statusID)
		}
	}

	return nil
}

// Validate validates the EmployeeQueryParams
func (p *EmployeeQueryParams) Validate() error {
	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	// Note: API documentation doesn't specify allowed sort fields for employees
	// So we don't validate the Sort field - let the API reject invalid values

	return nil
}

// Validate validates the InventoryQueryParams
func (p *InventoryQueryParams) Validate() error {
	// Validate required fields
	if p.Shop == 0 {
		return fmt.Errorf("shop is required for inventory queries")
	}
	if p.PartTypeID == 0 {
		return fmt.Errorf("partTypeId is required for inventory queries")
	}

	// Validate part type ID
	if p.PartTypeID != 1 && p.PartTypeID != 2 && p.PartTypeID != 5 {
		return fmt.Errorf("invalid partTypeId '%d': must be 1 (Part), 2 (Tire), or 5 (Battery)", p.PartTypeID)
	}

	direction, err := normalizeSortDirection(p.SortDirection)
	if err != nil {
		return err
	}
	p.SortDirection = direction
	p.Size = normalizePageSize(p.Size)

	// Sort accepts a comma separated list.
	validSorts := map[string]bool{"id": true, "name": true, "brand": true, "partNumber": true}
	if err := validateSortFields(p.Sort, validSorts, "id, name, brand, partNumber"); err != nil {
		return err
	}

	return nil
}
